package bridge

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

// RegistryPaths lists standard Windows registry locations for ASIO drivers.
var RegistryPaths = []struct {
	Key  registry.Key
	Path string
}{
	{registry.LOCAL_MACHINE, `SOFTWARE\ASIO`},
	{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\ASIO`},
}

// EnumerateInstalledDrivers scans the Windows registry for installed ASIO drivers.
func EnumerateInstalledDrivers() ([]DriverDesc, error) {
	var results []DriverDesc
	seenCLSIDs := make(map[string]bool)

	for _, entry := range RegistryPaths {
		k, err := registry.OpenKey(entry.Key, entry.Path, registry.READ)
		if err != nil {
			continue
		}

		subkeys, err := k.ReadSubKeyNames(-1)
		_ = k.Close()
		if err != nil {
			continue
		}

		for _, subkeyName := range subkeys {
			subKeyPath := entry.Path + `\` + subkeyName
			sk, err := registry.OpenKey(entry.Key, subKeyPath, registry.READ)
			if err != nil {
				continue
			}

			clsid, _, err := sk.GetStringValue("CLSID")
			if err != nil || strings.TrimSpace(clsid) == "" {
				_ = sk.Close()
				continue
			}
			clsid = strings.TrimSpace(clsid)

			// De-duplicate if present in both 64-bit and WOW6432 registry views
			if seenCLSIDs[strings.ToUpper(clsid)] {
				_ = sk.Close()
				continue
			}
			seenCLSIDs[strings.ToUpper(clsid)] = true

			desc, _, _ := sk.GetStringValue("Description")
			if strings.TrimSpace(desc) == "" {
				desc = subkeyName
			}
			_ = sk.Close()

			results = append(results, DriverDesc{
				ID:          subkeyName,
				Name:        subkeyName,
				Description: desc,
				CLSID:       clsid,
			})
		}
	}

	return results, nil
}
