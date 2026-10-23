package projectsetup

import _ "embed"

// generatedSetupScript is the same generic bootstrap contract shipped as the
// global setup template. Keeping it embedded makes --create-setup-script work
// even when the source tree or /usr/local/etc/update-cli is unavailable.
//
//go:embed setup-script-template.sh
var generatedSetupScript string
