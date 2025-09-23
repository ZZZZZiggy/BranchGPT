package services

import (
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"regexp"
	"strings"
)

func ExtractChapter(docID string, question string) (string, error) {
	re := regexp.MustCompile(`(?i)(section\s+)?([\d]+(?:\.[\d]+)*)(?:[\.\s]+([A-Z][A-Z_ ]*))?|(?i)(ABSTRACT|INTRODUCTION|CONCLUSION|RELATED\s+WORK|EXPERIMENTS|RESULTS|FUTURE\s+WORK)`)
	match := re.FindStringSubmatch(question)

	var sectionID string
	switch {
	case len(match) >= 4 && match[2] != "":
		sectionID = strings.TrimSpace(match[2])
		if match[3] != "" {
			// e.g., 7.SECURITY → 7.SECURITY
			sectionID += "." + strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(match[3]), " ", "_"))
		}
	case len(match) >= 5 && match[4] != "":
		sectionID = strings.ToUpper(strings.ReplaceAll(match[4], " ", "_"))
	default:
		return "", fmt.Errorf("未在问题中找到章节信息")
	}

	_, err := GetSection(docID, sectionID)
	if errors.Is(err, redis.Nil) {
		return "", fmt.Errorf("未找到章节 %s 的内容", sectionID)
	} else if err != nil {
		return "", fmt.Errorf("Redis 访问出错: %w", err)
	}

	return sectionID, nil
}
