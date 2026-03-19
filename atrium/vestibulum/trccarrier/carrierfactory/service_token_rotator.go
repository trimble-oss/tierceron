package carrierfactory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig/cache"
	"github.com/trimble-oss/tierceron/buildopts/cursoropts"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

const (
	pluginToolConfigPath      = "super-secrets/Restricted/PluginTool/config"
	rotationInterval          = 24 * time.Hour
	rotationLeadTime          = 14 * 24 * time.Hour
	serviceSecretValidityDays = 180
)

var serviceTokenRotatorSync sync.Map

func startDailyServiceTokenRotation(pluginEnvConfig map[string]any, logger *log.Logger) {
	if logger == nil {
		return
	}

	pluginName := cursoropts.BuildOptions.GetPluginName(true)
	if !strings.HasPrefix(pluginName, "trcsh-curator") {
		return
	}

	env, envOk := pluginEnvConfig["env"].(string)
	if !envOk || env == "" {
		logger.Println("Skipping service token rotation: missing env")
		return
	}

	if _, alreadyStarted := serviceTokenRotatorSync.LoadOrStore(env, true); alreadyStarted {
		return
	}

	tokenPtr := eUtils.RefMap(pluginEnvConfig, "tokenptr")
	vaultAddrPtr := eUtils.RefMap(pluginEnvConfig, "vaddress")
	if eUtils.RefLength(tokenPtr) < 5 || eUtils.RefLength(vaultAddrPtr) == 0 {
		logger.Printf("Skipping service token rotation for env %s: missing token or vault address\n", env)
		return
	}

	token := *tokenPtr
	vaultAddr := *vaultAddrPtr

	go func(rotationEnv string, rotationToken string, rotationAddr string) {
		defer func() {
			if r := recover(); r != nil {
				logger.Printf("Recovered panic in daily service token rotator for env %s: %v\n", rotationEnv, r)
			}
		}()

		runOnce := func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Printf("Recovered panic during service token rotation execution for env %s: %v\n", rotationEnv, r)
				}
			}()
			rotatePluginToolServiceToken(rotationEnv, rotationToken, rotationAddr, logger)
		}

		runOnce()

		ticker := time.NewTicker(rotationInterval)
		defer ticker.Stop()
		for range ticker.C {
			runOnce()
		}
	}(env, token, vaultAddr)
}

func rotatePluginToolServiceToken(env string, token string, vaultAddr string, logger *log.Logger) {
	currentTokenName := fmt.Sprintf("config_token_%s_unrestricted", env)
	tokenPtr := token
	pluginConfig := map[string]any{
		"env":           env,
		"vaddress":      vaultAddr,
		"tokenptr":      &tokenPtr,
		"exitOnFailure": false,
	}

	tokenCache := cache.NewTokenCache(currentTokenName, &tokenPtr, &vaultAddr)
	_, mod, vault, err := eUtils.InitVaultModForPlugin(pluginConfig, tokenCache, currentTokenName, logger)
	if err != nil {
		logger.Printf("Service token rotator init failed for env %s: %v\n", env, err)
		return
	}
	if vault != nil {
		defer vault.Close()
	}
	if mod != nil {
		defer mod.Release()
		// Ensure explicit paths are used when reading/writing PluginTool config.
		mod.SectionPath = ""
	}

	pluginToolConfig, err := mod.ReadData(pluginToolConfigPath)
	if err != nil {
		logger.Printf("Service token rotator failed reading PluginTool config for env %s: %v\n", env, err)
		return
	}

	tenantID, _ := pluginToolConfig["azureTenantId"].(string)
	clientID, _ := pluginToolConfig["azureClientId"].(string)
	clientSecret, _ := pluginToolConfig["azureClientSecret"].(string)
	if tenantID == "" || clientID == "" || clientSecret == "" {
		logger.Printf("Service token rotator missing required Azure credentials in PluginTool config for env %s\n", env)
		return
	}

	if !shouldRotateNow(pluginToolConfig) {
		logger.Printf("Service token rotator skipped for env %s: existing secret is not close to expiry\n", env)
		return
	}

	graphToken, err := getGraphAccessToken(tenantID, clientID, clientSecret)
	if err != nil {
		logger.Printf("Service token rotator failed acquiring Graph token for env %s: %v\n", env, err)
		return
	}

	applicationID, err := getApplicationObjectID(graphToken, clientID)
	if err != nil {
		logger.Printf("Service token rotator failed resolving application object id for env %s: %v\n", env, err)
		return
	}

	newSecret, newKeyID, newExpiresOn, err := addApplicationPassword(graphToken, applicationID)
	if err != nil {
		logger.Printf("Service token rotator addPassword failed for env %s: %v\n", env, err)
		return
	}

	oldKeyID, _ := pluginToolConfig["azureClientSecretKeyId"].(string)
	pluginToolConfig["azureClientSecret"] = newSecret
	pluginToolConfig["azureClientSecretKeyId"] = newKeyID
	pluginToolConfig["azureClientSecretExpiresOn"] = newExpiresOn
	pluginToolConfig["azureClientSecretLastRotated"] = time.Now().UTC().Format(time.RFC3339)

	_, err = mod.Write(pluginToolConfigPath, pluginToolConfig, logger)
	if err != nil {
		logger.Printf("Service token rotator failed writing PluginTool config for env %s: %v\n", env, err)
		return
	}

	if oldKeyID != "" && oldKeyID != newKeyID {
		if removeErr := removeApplicationPassword(graphToken, applicationID, oldKeyID); removeErr != nil {
			logger.Printf("Service token rotator warning for env %s: unable to remove previous secret key id %s: %v\n", env, oldKeyID, removeErr)
		}
	}

	logger.Printf("Service token rotator succeeded for env %s; next expiry %s\n", env, newExpiresOn)
}

func shouldRotateNow(pluginToolConfig map[string]any) bool {
	expiresOn, ok := pluginToolConfig["azureClientSecretExpiresOn"].(string)
	if !ok || expiresOn == "" {
		return true
	}
	expiry, err := time.Parse(time.RFC3339, expiresOn)
	if err != nil {
		return true
	}
	return time.Now().UTC().After(expiry.Add(-rotationLeadTime))
}

func getGraphAccessToken(tenantID string, clientID string, clientSecret string) (string, error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("scope", "https://graph.microsoft.com/.default")

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("graph token request failed with status %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", err
	}
	if tokenResp.AccessToken == "" {
		return "", errors.New("graph token response missing access_token")
	}
	return tokenResp.AccessToken, nil
}

func getApplicationObjectID(graphToken string, clientID string) (string, error) {
	query := fmt.Sprintf("https://graph.microsoft.com/v1.0/applications?$filter=appId%%20eq%%20'%s'&$select=id", clientID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, query, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+graphToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("application lookup failed with status %d", resp.StatusCode)
	}

	var appResp struct {
		Value []struct {
			ID string `json:"id"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &appResp); err != nil {
		return "", err
	}
	if len(appResp.Value) == 0 || appResp.Value[0].ID == "" {
		return "", errors.New("application lookup returned no result")
	}
	return appResp.Value[0].ID, nil
}

func addApplicationPassword(graphToken string, applicationID string) (string, string, string, error) {
	rotateUntil := time.Now().UTC().Add(serviceSecretValidityDays * 24 * time.Hour)
	payload := map[string]any{
		"passwordCredential": map[string]any{
			"displayName": "tierceron-auto-rotated",
			"endDateTime": rotateUntil.Format(time.RFC3339),
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", "", "", err
	}

	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/applications/%s/addPassword", applicationID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+graphToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", "", fmt.Errorf("addPassword failed with status %d", resp.StatusCode)
	}

	var addResp struct {
		SecretText  string `json:"secretText"`
		KeyID       string `json:"keyId"`
		EndDateTime string `json:"endDateTime"`
	}
	if err := json.Unmarshal(body, &addResp); err != nil {
		return "", "", "", err
	}
	if addResp.SecretText == "" || addResp.KeyID == "" {
		return "", "", "", errors.New("addPassword response missing secretText or keyId")
	}
	return addResp.SecretText, addResp.KeyID, addResp.EndDateTime, nil
}

func removeApplicationPassword(graphToken string, applicationID string, keyID string) error {
	payload := map[string]any{"keyId": keyID}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://graph.microsoft.com/v1.0/applications/%s/removePassword", applicationID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+graphToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("removePassword failed with status %d", resp.StatusCode)
	}

	return nil
}
