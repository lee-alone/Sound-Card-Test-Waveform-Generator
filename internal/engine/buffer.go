package engine

// BufferPool maintains pre-allocated memory slices for real-time rendering.
// This guarantees zero heap allocations on the audio streaming hot path.
type BufferPool struct {
	Channels      [][]float64
	RawBytes      []byte
	NumFrames     int
	NumChannels   int
	BytesPerFrame int
}

// NewBufferPool allocates all required sample and byte slices ahead of time.
func NewBufferPool(numFrames int, numChannels int, bytesPerSample int) *BufferPool {
	if numFrames <= 0 {
		numFrames = 1024
	}
	if numChannels <= 0 {
		numChannels = 2
	}
	if bytesPerSample <= 0 {
		bytesPerSample = 2
	}

	channels := make([][]float64, numChannels)
	for ch := 0; ch < numChannels; ch++ {
		channels[ch] = make([]float64, numFrames)
	}

	bytesPerFrame := bytesPerSample * numChannels
	totalBytes := numFrames * bytesPerFrame
	rawBytes := make([]byte, totalBytes)

	return &BufferPool{
		Channels:      channels,
		RawBytes:      rawBytes,
		NumFrames:     numFrames,
		NumChannels:   numChannels,
		BytesPerFrame: bytesPerFrame,
	}
}

// Zero fills all channel buffers and raw byte buffers with zeros.
func (bp *BufferPool) Zero() {
	for ch := 0; ch < bp.NumChannels; ch++ {
		for i := 0; i < bp.NumFrames; i++ {
			bp.Channels[ch][i] = 0.0
		}
	}
	for i := 0; i < len(bp.RawBytes); i++ {
		bp.RawBytes[i] = 0
	}
}
