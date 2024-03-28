package recorder

type RecorderManager struct {
	lookup map[string]*RecorderInfo
}
type (
	Option       func(*RecorderManager)
	RecorderInfo struct {
		Key string
	}
)

func NewRecorder(opts ...Option) *RecorderManager {
	ret := &RecorderManager{
		lookup: make(map[string]*RecorderInfo),
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func (rm *RecorderManager) AddRecorder(recorder *RecorderInfo) {
	if _, ok := rm.lookup[recorder.Key]; ok {
		return
	}
	rm.lookup[recorder.Key] = recorder
}
