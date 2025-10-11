package asynq

const (
	UploadVideoTask      = "video:upload"
	UploadVideo1080pTask = "video:upload_1080p"
	UploadVideo720pTask  = "video:upload_720p"
	UploadVideo480pTask  = "video:upload_480p"
	UploadVideo360pTask  = "video:upload_360p"
	UploadVideo240pTask  = "video:upload_240p"

	TranscodingTask      = "video:transcode"
	Transcoding1080pTask = "video:transcode_1080p"
	Transcoding720pTask  = "video:transcode_720p"
	Transcoding480pTask  = "video:transcode_480p"
	Transcoding360pTask  = "video:transcode_360p"
	Transcoding240pTask  = "video:transcode_240p"
)

type UploadVideoPayload struct {
	VideoID  string // internal ID
	FilePath string // path lokal atau remote
	FileName string
	UserID   string
}

type TranscodePayload struct {
	VideoID    string
	InputPath  string
	Resolution string // "1080p", "720p", etc
	OutputPath string // optional jika mau custom naming
}
