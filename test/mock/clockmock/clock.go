package clockmock

import "time"

type Impl struct{}

func New() *Impl {
	return &Impl{}
}

func (c *Impl) Now() time.Time {
	return time.Time{}
}
