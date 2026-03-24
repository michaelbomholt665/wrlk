package ext

import "github.com/michaelbomholt665/wrlk/internal/router"

var optionalExtensions = []router.Extension{
	&optionalExample{},
}
