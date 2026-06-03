package memory

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
)

var loaderLog = logger.ForComponent("ai.memory.loader")

// LoadFromDir scans a directory for .md memory files and parses them.
func LoadFromDir(dir string) ([]*Memory, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		loaderLog.Error("扫描记忆目录失败", "dir", dir, "error", err)
		return nil, err
	}

	loaderLog.Info("开始扫描记忆目录", "dir", dir, "entries", len(entries))

	var memories []*Memory
	skipped := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			skipped++
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		m, err := parseFile(fullPath)
		if err != nil {
			loaderLog.Warn("解析记忆文件失败，跳过", "file", entry.Name(), "error", err)
			continue
		}
		memories = append(memories, m)
		loaderLog.Info("记忆文件解析成功", "file", entry.Name(), "name", m.Name, "type", string(m.Type))
	}

	loaderLog.Info("记忆目录扫描完成", "loaded", len(memories), "skipped_non_md", skipped)
	return memories, nil
}

func parseFile(path string) (*Memory, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	if !scanner.Scan() {
		return nil, os.ErrInvalid
	}
	firstLine := strings.TrimSpace(scanner.Text())
	if firstLine != "---" {
		return nil, os.ErrInvalid
	}

	fm := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			fm[key] = val
		}
	}

	var body strings.Builder
	for scanner.Scan() {
		body.WriteString(scanner.Text())
		body.WriteByte('\n')
	}

	name := fm["name"]
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), ".md")
	}

	memType := MemoryType(fm["type"])
	if memType == "" {
		memType = TypeReference
	}

	now := time.Now()
	return &Memory{
		Name:        name,
		Description: fm["description"],
		Type:        memType,
		Content:     strings.TrimSpace(body.String()),
		Source:      "file",
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
