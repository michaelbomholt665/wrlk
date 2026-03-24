package ext

import "policycheck/internal/router"

var optionalExtensions = []router.Extension{
	&telemetryExample{},
}
