package post_service

type MediaAttachment struct {
	MediaID   string
	URL       string
	MediaType string
}

type Post struct {
	ID        string
	UserID    string
	Content   string
	Media     *MediaAttachment
	CreatedAt int64
}

type CreatePostDTO struct {
	UserID  string
	Content string
	Media   *MediaAttachment
}

type CreatePostResponse struct {
	PostID    string
	CreatedAt int
}

type PresignedUploadUrlDTO struct {
	FileExtension string
	MimeType      string
}

type GetPresignedUploadUrlResponse struct {
	UploadUrl string
	MediaUrl  string
	MediaID   string
}
