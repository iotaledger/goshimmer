package database

type logger struct{}

func (this *logger) Errorf(string, ...interface{}) {
	// disable logging
}

func (this *logger) Infof(string, ...interface{}) {
	// disable logging
}

func (this *logger) Warningf(string, ...interface{}) {
	// disable logging
}

func (this *logger) Debugf(string, ...interface{}) {
	// disable logging
}
