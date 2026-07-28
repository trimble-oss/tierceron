package cert

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig/cache"
	"github.com/trimble-oss/tierceron-core/v2/prod"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	vcutils "github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase/utils"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	"github.com/trimble-oss/tierceron/pkg/validator"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

var certPathLocks sync.Map

func getCertPathLock(certPath string) *sync.Mutex {
	pathLock, _ := certPathLocks.LoadOrStore(certPath, &sync.Mutex{})
	return pathLock.(*sync.Mutex)
}

// IsCertValidBySupportedDomains accepts a certificate
func IsCertValidBySupportedDomains(byteCert []byte,
	certValidationHelper func(cert *x509.Certificate, host string, selfSignedOk bool) (bool, error),
) (bool, *x509.Certificate, error) {
	var ok bool
	var err error
	block, _ := pem.Decode(byteCert)
	if block == nil {
		return false, nil, errors.New("failed to parse certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, nil, errors.New("failed to parse certificate: " + err.Error())
	}

	for _, domain := range coreopts.BuildOptions.GetSupportedDomains(prod.IsProd()) {
		if ok, err = certValidationHelper(cert, domain, prod.IsProd()); ok {
			return ok, cert, err
		}
	}
	return ok, cert, err
}

func LoadCertComponent(driverConfig *config.DriverConfig, goMod *helperkv.Modifier, certPath string) ([]byte, error) {
	pathLock := getCertPathLock(certPath)
	pathLock.Lock()
	defer pathLock.Unlock()
	if driverConfig.CoreConfig.CertCache != nil {
		if v, ok := driverConfig.CoreConfig.CertCache.Get(certPath); ok && v != nil && v.CertBytes != nil {
			return *v.CertBytes, nil
		}
	}
	cert_ps := strings.Split(certPath, "/")
	if len(cert_ps) != 2 {
		return nil, errors.New("unable to process cert")
	}
	certBasis := strings.Split(cert_ps[1], ".")
	templatePath := "./trc_templates/" + certPath

	_, configuredCert, _, err := vcutils.ConfigTemplate(driverConfig, goMod, templatePath, true, cert_ps[0], certBasis[0], true, true)
	if err != nil {
		eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
		return nil, err
	}
	if len(configuredCert) < 2 {
		return nil, errors.New("No certificate data found")
	}
	certBytes := []byte(configuredCert[1])
	return certBytes, nil
}

func AddToCache(path string, driverConfig *config.DriverConfig, mod *kv.Modifier) (*[]byte, error) {
	// Trim path
	if driverConfig.CoreConfig.CertCache == nil {
		driverConfig.CoreConfig.CertCache = cache.NewCertCache()
	}
	if v, ok := driverConfig.CoreConfig.CertCache.Get(path); ok {
		driverConfig.CoreConfig.WantCerts = false
		return v.CertBytes, nil
	}
	certPath := strings.TrimPrefix(path, "Common/")
	certPath = strings.TrimSuffix(certPath, ".crt.mf.tmpl")
	certPath = strings.TrimSuffix(certPath, ".key.mf.tmpl")
	certPath = strings.TrimSuffix(certPath, ".pem.mf.tmpl")
	certPath = strings.TrimSuffix(certPath, ".asc.mf.tmpl")
	metadata, err := mod.ReadMetadata(fmt.Sprintf("values/%s", certPath), driverConfig.CoreConfig.Log)
	if err != nil {
		eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
		return nil, err
	}
	if t, ok := metadata["created_time"]; ok {
		configuredCert, err := LoadCertComponent(driverConfig,
			mod,
			path)
		if err != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
			return nil, err
		}
		valid := false
		var cert *x509.Certificate
		var certNotAfter *time.Time
		if strings.HasSuffix(path, ".crt.mf.tmpl") {
			valid, cert, err = IsCertValidBySupportedDomains(configuredCert, validator.VerifyCertificate)
			if err != nil || cert == nil {
				eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
				return nil, err
			}
			certNotAfter = &cert.NotAfter
		} else {
			valid = true
			certNotAfter = &time.Time{}
		}

		if valid {
			var zeroTime time.Time
			certSha256 := ""
			if len(configuredCert) > 0 {
				certHash := sha256.Sum256(configuredCert)
				certSha256 = hex.EncodeToString(certHash[:])
			} else {
				driverConfig.CoreConfig.Log.Println("Empty cert bytes loaded for adding to cert cache")
			}
			driverConfig.CoreConfig.CertCache.Set(path, &cache.CertValue{
				CreatedTime: t,
				CertBytes:   &configuredCert,
				NotAfter:    certNotAfter,
				LastUpdate:  &zeroTime,
				Sha256:      certSha256,
			})

			driverConfig.CoreConfig.WantCerts = false

			return &configuredCert, nil
		} else {
			driverConfig.CoreConfig.Log.Println("Invalid cert")
			return nil, errors.New("invalid cert")
		}
	}
	driverConfig.CoreConfig.Log.Println("Unable to access created time for cert.")
	return nil, errors.New("no created time for cert")
}
