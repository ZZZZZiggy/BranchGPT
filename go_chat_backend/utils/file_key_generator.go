package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type FileKeyStrategy string

const (
	StrategyHashBased FileKeyStrategy = "hash_based"
	StrategyDateBased FileKeyStrategy = "date_based"
	StrategyUserBased FileKeyStrategy = "user_based"
)

type FileKeyGenerator struct {
	strategy   FileKeyStrategy
	prefix     string
	maxNameLen int
}

func NewFileKeyGenerator(strategy FileKeyStrategy, prefix string) *FileKeyGenerator {
	return &FileKeyGenerator{
		strategy:   strategy,
		prefix:     prefix,
		maxNameLen: 50,
	}
}

func (fkg *FileKeyGenerator) GenerateFileKey(filename, userID string) string {
	switch fkg.strategy {
	case StrategyHashBased:
		return fkg.generateHashBasedKey(filename, userID)
	case StrategyDateBased:
		return fkg.generateDateBasedKey(filename, userID)
	case StrategyUserBased:
		return fkg.generateUserBasedKey(filename, userID)
	default:
		return fkg.generateTimestampUUIDKey(filename)
	}
}

// 1. 时间戳 + UUID策略（默认）
func (fkg *FileKeyGenerator) generateTimestampUUIDKey(filename string) string {
	timestamp := time.Now().Unix()
	uid := uuid.New().String()
	cleanName := fkg.cleanFilename(filename)

	return fmt.Sprintf("%s/%d_%s_%s", fkg.prefix, timestamp, uid, cleanName)
}

// 2. 基于内容哈希的策略（避免重复上传）
func (fkg *FileKeyGenerator) generateHashBasedKey(filename, userID string) string {
	// 使用文件名+用户ID+时间生成哈希
	content := fmt.Sprintf("%s_%s_%d", filename, userID, time.Now().UnixNano())
	hash := md5.Sum([]byte(content))
	hashStr := hex.EncodeToString(hash[:])

	ext := filepath.Ext(filename)
	return fmt.Sprintf("%s/hash_%s%s", fkg.prefix, hashStr, ext)
}

// 3. 基于日期的分层存储策略
func (fkg *FileKeyGenerator) generateDateBasedKey(filename, userID string) string {
	now := time.Now()
	year := now.Format("2006")
	month := now.Format("01")
	day := now.Format("02")

	uid := uuid.New().String()[:8] // 短UUID
	cleanName := fkg.cleanFilename(filename)

	return fmt.Sprintf("%s/%s/%s/%s/%s_%s", fkg.prefix, year, month, day, uid, cleanName)
}

// 4. 基于用户的策略
func (fkg *FileKeyGenerator) generateUserBasedKey(filename, userID string) string {
	timestamp := time.Now().Unix()
	uid := uuid.New().String()[:12] // 中等长度UUID
	cleanName := fkg.cleanFilename(filename)

	// 用户ID哈希化以保护隐私
	userHash := fkg.hashString(userID)[:8]

	return fmt.Sprintf("%s/users/%s/%d_%s_%s", fkg.prefix, userHash, timestamp, uid, cleanName)
}

// 文件名清理函数（增强版）
func (fkg *FileKeyGenerator) cleanFilename(filename string) string {
	// 获取文件扩展名
	ext := strings.ToLower(filepath.Ext(filename))
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))

	// 处理中文和特殊字符
	cleanBase := fkg.sanitizeFilename(baseName)

	// 限制长度
	if len(cleanBase) > fkg.maxNameLen {
		// 智能截取：优先保留前面的内容
		cleanBase = cleanBase[:fkg.maxNameLen]
		// 确保不在中文字符中间截断
		cleanBase = fkg.ensureValidUTF8End(cleanBase)
	}

	// 如果清理后为空，使用默认名称
	if cleanBase == "" || cleanBase == "_" {
		cleanBase = "document"
	}

	return cleanBase + ext
}

// 文件名净化
func (fkg *FileKeyGenerator) sanitizeFilename(name string) string {
	// 替换空格为下划线
	name = strings.ReplaceAll(name, " ", "_")

	// 移除危险字符
	dangerousChars := `[<>:"/\\|?*]`
	reg := regexp.MustCompile(dangerousChars)
	name = reg.ReplaceAllString(name, "")

	// 只保留安全字符：字母、数字、中文、下划线、连字符、点号
	safePattern := regexp.MustCompile(`[^\p{L}\p{N}_\-.]`)
	name = safePattern.ReplaceAllString(name, "_")

	// 清理连续的特殊字符
	name = regexp.MustCompile(`[_\-\.]{2,}`).ReplaceAllString(name, "_")

	// 移除首尾特殊字符
	name = strings.Trim(name, "_-.")

	return name
}

// 确保UTF-8字符完整性
func (fkg *FileKeyGenerator) ensureValidUTF8End(s string) string {
	if len(s) == 0 {
		return s
	}

	// 检查最后几个字节是否是完整的UTF-8字符
	for i := len(s) - 1; i >= 0 && i >= len(s)-4; i-- {
		if s[i]&0x80 == 0 { // ASCII字符
			return s
		}
		if s[i]&0xC0 == 0xC0 { // UTF-8字符开始
			return s[:i]
		}
	}
	return s
}

// 辅助函数：生成字符串哈希
func (fkg *FileKeyGenerator) hashString(s string) string {
	hash := md5.Sum([]byte(s))
	return hex.EncodeToString(hash[:])
}
