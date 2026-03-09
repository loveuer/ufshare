package model

// Setting 系统配置项，以 key-value 形式存储在数据库
type Setting struct {
	Key   string `json:"key"   gorm:"primaryKey;size:128"`
	Value string `json:"value" gorm:"type:text"`
}

func (Setting) TableName() string { return "settings" }
