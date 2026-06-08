package cert

import (
	"errors"
	"strings"

	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig/cache"
	vcutils "github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase/utils"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func LoadCertComponent(driverConfig *config.DriverConfig, goMod *helperkv.Modifier, certPath string) ([]byte, error) {
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
	driverConfig.CoreConfig.WantCerts = true
	_, configuredCert, _, err := vcutils.ConfigTemplate(driverConfig, goMod, templatePath, true, cert_ps[0], certBasis[0], true, true)
	if err != nil {
		eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
		return nil, err
	}
	if len(configuredCert) < 2 {
		return nil, errors.New("No certificate data found")
	}
	certBytes := []byte(configuredCert[1])
	if driverConfig.CoreConfig.CertCache == nil {
		driverConfig.CoreConfig.CertCache = cache.NewCertCache()
	}
	driverConfig.CoreConfig.CertCache.Set(certPath, &cache.CertValue{
		CertBytes: &certBytes,
	})
	return certBytes, nil
}
