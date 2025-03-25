package schema

import "github.com/netcracker/qubership-core-lib-go/v3/logging"

var log logging.Logger

func init() {
	log = logging.GetLogger("schema-migration")
}
