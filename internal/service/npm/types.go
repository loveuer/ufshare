package npm

import "encoding/json"

// ── 请求体类型（npm 协议） ────────────────────────────────────────────────────

// PublishBody 是 npm publish 发送的 PUT /:package 请求体
type PublishBody struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Readme      string                     `json:"readme"`
	DistTags    map[string]string          `json:"dist-tags"`
	Versions    map[string]json.RawMessage `json:"versions"`
	Attachments map[string]Attachment      `json:"_attachments"`
}

// Attachment 是 _attachments 中每个 tarball 的描述
type Attachment struct {
	ContentType string `json:"content_type"`
	Data        string `json:"data"`   // base64 编码的 tarball 内容
	Length      int64  `json:"length"` // 字节数
}

// ── 响应体类型（npm 协议） ────────────────────────────────────────────────────

// Packument 是 GET /:package 的响应体（npm 格式）
type Packument struct {
	ID          string                     `json:"_id"`
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Readme      string                     `json:"readme,omitempty"`
	DistTags    map[string]string          `json:"dist-tags"`
	Versions    map[string]json.RawMessage `json:"versions"`
	Time        map[string]string          `json:"time,omitempty"`
}
