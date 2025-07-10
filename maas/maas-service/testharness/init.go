package testharness

import (
	"github.com/knadh/koanf/providers/confmap"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
)

var log = logging.GetLogger("testharness")

func init() {
	configloader.Init(&configloader.PropertySource{Provider: configloader.AsPropertyProvider(confmap.Provider(map[string]interface{}{
		"db.cipher.key": "thisis32bitlongpassphraseimusing",
	}, "."))})
}
