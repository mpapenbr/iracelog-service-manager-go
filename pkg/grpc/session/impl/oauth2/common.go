package oauth2

import "time"

type (
	myRefresher struct {
		id    string
		timer *time.Timer
	}
)
