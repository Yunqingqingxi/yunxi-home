package toolreg

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/register"
	"github.com/Yunqingqingxi/yunxi-home/internal/nas"
)

// RegisterFileTools 注册 NAS 文件管理工具到 AI 注册中心
// fs 应为沙箱文件服务，确保 AI 只能在限定目录操作
func RegisterFileTools(r *register.Registry, fs nas.FileService) {
	if fs == nil {
		return
	}
	sandboxRoot := fs.SandboxRoot()

	// ── 列出目录 ────────────────────────────────────
	r.Register(&base.ToolDef{
		Name:        "file_list",
		Description: fmt.Sprintf("列出沙箱目录中的文件和子目录。沙箱根: %s。当用户问'查看文件'、'列出目录'、'有什么文件'时调用。", sandboxRoot),
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"path": {Type: "string", Description: "目录路径，相对于沙箱根目录，例如 / 或 /downloads"},
			},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			path := "/"
			if v, ok := args["path"].(string); ok && v != "" {
				path = v
			}
			files, err := fs.ListDir(path)
			if err != nil {
				return "", fmt.Errorf("列出目录失败: %w", err)
			}
			return ToJSON(map[string]any{
				"path":  path,
				"count": len(files),
				"files": files,
			})
		},
	})

	// ── 创建目录 ────────────────────────────────────
	r.Register(&base.ToolDef{
		Name:        "file_mkdir",
		Description: "在沙箱中创建新目录。当用户说'创建文件夹'时调用。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"path": {Type: "string", Description: "要创建的目录路径，例如 /downloads/movies"},
			},
			Required: []string{"path"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				return "", fmt.Errorf("请指定目录路径")
			}
			if err := fs.Mkdir(path); err != nil {
				return "", err
			}
			return fmt.Sprintf("目录已创建: %s", path), nil
		},
	})

	// ── 删除文件/目录 (需确认，支持批量) ────────────
	r.Register(&base.ToolDef{
		Name:        "file_delete",
		RiskLevel:   "dangerous",
		Description: "删除沙箱中的文件或目录。⚠️ 危险操作，需设置 confirm=true 才会执行。支持批量: 传 paths 数组一次删除多个。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"path":    {Type: "string", Description: "要删除的单个文件或目录路径"},
				"paths":   {Type: "array", Description: "要批量删除的路径数组，优先于 path"},
				"confirm": {Type: "boolean", Description: "确认执行删除。设为 true 才会真正删除，否则仅返回即将删除的内容预览。"},
			},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			confirm := false
			if v, ok := args["confirm"].(bool); ok {
				confirm = v
			}

			// 收集要删除的路径
			var paths []string
			if arr, ok := args["paths"].([]any); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						paths = append(paths, s)
					}
				}
			} else if p, _ := args["path"].(string); p != "" {
				paths = []string{p}
			}
			if len(paths) == 0 {
				return "", fmt.Errorf("请提供 path 或 paths")
			}

			if !confirm {
				var preview []map[string]any
				for _, p := range paths {
					files, _ := fs.ListDir(p)
					preview = append(preview, map[string]any{"path": p, "contains": len(files)})
				}
				return ToJSON(map[string]any{
					"warning": fmt.Sprintf("即将删除 %d 个项目。请设置 confirm=true 确认。", len(paths)),
					"preview": preview,
					"action":  "delete",
				})
			}

			var results []map[string]any
			for _, p := range paths {
				err := fs.Delete(p)
				r := map[string]any{"path": p, "success": err == nil}
				if err != nil {
					r["error"] = err.Error()
				}
				results = append(results, r)
			}
			return ToJSON(map[string]any{
				"message": fmt.Sprintf("已删除 %d 个项目", len(paths)),
				"results": results,
			})
		},
	})

	// ── 下载/读取文件 ────────────────────────────────
	r.Register(&base.ToolDef{
		Name:        "file_read",
		Description: "读取沙箱中文本文件的内容。支持文本文件和 base64 编码的二进制文件预览。当用户说'查看文件内容'、'读取配置'时调用。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"path":   {Type: "string", Description: "文件路径"},
				"base64": {Type: "boolean", Description: "是否返回 base64 编码（用于二进制文件），默认 false 返回文本"},
				"offset": {Type: "integer", Description: "从第几个字节开始读取，默认 0"},
				"length": {Type: "integer", Description: "读取字节数，默认 4096，最大 65536"},
			},
			Required: []string{"path"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			useBase64 := false
			if v, ok := args["base64"].(bool); ok {
				useBase64 = v
			}
			offset := int64(GetInt(args, "offset", 0))
			length := GetInt(args, "length", 4096)
			if length > 65536 {
				length = 65536
			}

			reader, stat, err := fs.OpenFile(path)
			if err != nil {
				return "", err
			}
			defer reader.Close()

			// 跳过 offset
			if offset > 0 {
				if seeker, ok := reader.(io.Seeker); ok {
					seeker.Seek(offset, io.SeekStart)
				} else {
					io.CopyN(io.Discard, reader, offset)
				}
			}

			buf := make([]byte, length)
			n, _ := io.ReadFull(reader, buf)
			if n == 0 {
				return ToJSON(map[string]any{
					"path": path,
					"size": stat.Size(),
					"read": 0,
				})
			}

			data := buf[:n]
			if useBase64 {
				return ToJSON(map[string]any{
					"path":   path,
					"size":   stat.Size(),
					"read":   n,
					"offset": offset,
					"base64": base64.StdEncoding.EncodeToString(data),
				})
			}

			// 检测是否为文本
			text := string(data)
			if !isPrintable(text) {
				return ToJSON(map[string]any{
					"path":    path,
					"size":    stat.Size(),
					"warning": "文件可能为二进制格式，请使用 base64=true 读取",
					"preview": text[:min(len(text), 200)],
				})
			}

			return ToJSON(map[string]any{
				"path":    path,
				"size":    stat.Size(),
				"read":    n,
				"content": text,
			})
		},
	})

	// ── 上传/写入文件 ────────────────────────────────
	r.Register(&base.ToolDef{
		Name:        "file_write",
		Description: "在沙箱中写入文件内容（创建或覆盖）。⚠️ 覆盖已有文件需设置 confirm=true。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"path":    {Type: "string", Description: "文件路径，例如 /downloads/notes.txt"},
				"content": {Type: "string", Description: "要写入的文本内容"},
				"base64":  {Type: "string", Description: "base64 编码的二进制内容（与 content 二选一）"},
				"confirm": {Type: "boolean", Description: "如果文件已存在，需设为 true 确认覆盖"},
			},
			Required: []string{"path"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			confirm := false
			if v, ok := args["confirm"].(bool); ok {
				confirm = v
			}

			// 检查是否已存在
			if fs.Exists(path) && !confirm {
				return ToJSON(map[string]any{
					"warning": fmt.Sprintf("文件 %s 已存在。请设置 confirm=true 确认覆盖。", path),
					"path":    path,
					"action":  "write",
				})
			}

			// 按路径分隔出 dir 和 filename
			dir := "/"
			filename := path
			if idx := strings.LastIndex(path, "/"); idx >= 0 {
				dir = path[:idx]
				if dir == "" {
					dir = "/"
				}
				filename = path[idx+1:]
			}

			var reader io.Reader
			if b64, ok := args["base64"].(string); ok && b64 != "" {
				data, err := base64.StdEncoding.DecodeString(b64)
				if err != nil {
					return "", fmt.Errorf("base64 解码失败: %w", err)
				}
				reader = bytes.NewReader(data)
			} else if content, ok := args["content"].(string); ok {
				reader = strings.NewReader(content)
			} else {
				return "", fmt.Errorf("请提供 content 或 base64 参数")
			}

			savedPath, err := fs.SaveFile(dir, filename, reader)
			if err != nil {
				return "", fmt.Errorf("写入失败: %w", err)
			}
			return ToJSON(map[string]any{
				"message": "文件已写入",
				"path":    filepath.ToSlash(savedPath),
			})
		},
	})

	// ── 文件搜索 ────────────────────────────────────
	r.Register(&base.ToolDef{
		Name:        "file_search",
		Description: "在沙箱目录中搜索文件（按名称匹配，支持递归搜索）。当用户说'搜索文件'、'查找xxx'时调用。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"query":     {Type: "string", Description: "搜索关键词"},
				"path":      {Type: "string", Description: "搜索起始目录，默认 /"},
				"recursive": {Type: "boolean", Description: "是否递归搜索子目录，默认 false"},
				"max_depth": {Type: "integer", Description: "递归最大深度，默认 3，最大 10"},
			},
			Required: []string{"query"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			query, _ := args["query"].(string)
			path := "/"
			if v, ok := args["path"].(string); ok && v != "" {
				path = v
			}
			recursive := false
			if v, ok := args["recursive"].(bool); ok {
				recursive = v
			}
			maxDepth := GetInt(args, "max_depth", 3)
			if maxDepth < 1 {
				maxDepth = 1
			}
			if maxDepth > 10 {
				maxDepth = 10
			}

			files, err := fs.SearchFiles(path, query, recursive, maxDepth)
			if err != nil {
				return "", err
			}

			return ToJSON(map[string]any{
				"query":     strings.ToLower(query),
				"path":      path,
				"recursive": recursive,
				"max_depth": maxDepth,
				"count":     len(files),
				"results":   files,
			})
		},
	})

	// ── 磁盘信息 ────────────────────────────────────
	r.Register(&base.ToolDef{
		Name:        "file_disk_info",
		Description: "获取沙箱所在磁盘的使用情况（总容量、已用、剩余）。当用户问'磁盘空间'时调用。",
		Parameters: base.ToolParams{
			Type:       "object",
			Properties: map[string]base.ParamProp{},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			info, err := fs.GetDiskInfo("/")
			if err != nil {
				return "", err
			}
			return ToJSON(map[string]any{
				"total_gb":   float64(info.Total) / 1024 / 1024 / 1024,
				"used_gb":    float64(info.Used) / 1024 / 1024 / 1024,
				"free_gb":    float64(info.Free) / 1024 / 1024 / 1024,
				"used_pct":   info.UsedPct,
				"sandbox":    fs.IsSandbox(),
				"sandbox_root": fs.SandboxRoot(),
			})
		},
	})

	// ── 沙箱信息 ────────────────────────────────────
	r.Register(&base.ToolDef{
		Name:        "file_sandbox_info",
		Description: "获取 AI 文件沙箱的当前状态和限制信息。当用户问'沙箱状态'或'我可以访问哪些文件'时调用。",
		Parameters: base.ToolParams{
			Type:       "object",
			Properties: map[string]base.ParamProp{},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			info := fs.SandboxInfo()
			info["description"] = "AI 文件操作限定在沙箱根目录内，无法访问系统文件或其他服务数据。"
			info["allowed_operations"] = []string{"list", "read", "write", "mkdir", "delete", "search", "diskinfo"}
			info["danger_ops_require_confirm"] = []string{"delete", "write(overwrite)"}
			info["shared_with"] = []string{"Nextcloud (外部存储)", "Samba 共享"}
			return ToJSON(info)
			},
})

// ── 下载文件 ────────────────────────────────────
r.Register(&base.ToolDef{
	Name:        "file_download",
	Description: "从互联网 URL 下载文件到沙箱。支持批量: 传 items: [{url, filename?, dir?}] 一次下载多个。",
		Background:  true,
	Parameters: base.ToolParams{
		Type: "object",
		Properties: map[string]base.ParamProp{
			"url":      {Type: "string", Description: "单个下载 URL"},
			"dir":      {Type: "string", Description: "保存目录，默认 /"},
			"filename": {Type: "string", Description: "保存的文件名"},
			"items":    {Type: "array", Description: "批量: [{url, filename?, dir?}]，优先于 url"},
		},
	},
	Handler: func(ctx context.Context, args map[string]any) (string, error) {
		type dl struct{ url, dir, filename string }
		var items []dl
		if arr, ok := args["items"].([]any); ok {
			for _, v := range arr {
				if m, ok := v.(map[string]any); ok {
					it := dl{url: m["url"].(string), dir: m["dir"].(string), filename: m["filename"].(string)}
					items = append(items, it)
				}
			}
		} else {
			url, _ := args["url"].(string)
			if url != "" {
				d, _ := args["dir"].(string)
				f, _ := args["filename"].(string)
				items = []dl{{url, d, f}}
			}
		}
		if len(items) == 0 {
			return "", fmt.Errorf("请提供 url 或 items")
		}

		var results []map[string]any
		client := &http.Client{Timeout: 300 * time.Second}
		for _, it := range items {
			dir := it.dir
			if dir == "" { dir = "/" }
			filename := it.filename
			if filename == "" {
				parts := strings.Split(it.url, "/")
				filename = parts[len(parts)-1]
				if idx := strings.Index(filename, "?"); idx >= 0 {
					filename = filename[:idx]
				}
				if filename == "" { filename = "download" }
			}
			req, _ := http.NewRequestWithContext(ctx, "GET", it.url, nil)
			resp, err := client.Do(req)
			if err != nil {
				results = append(results, map[string]any{"url": it.url, "success": false, "error": err.Error()})
				continue
			}
			if resp.StatusCode >= 400 {
				resp.Body.Close()
				results = append(results, map[string]any{"url": it.url, "success": false, "error": fmt.Sprintf("HTTP %d", resp.StatusCode)})
				continue
			}
			savedPath, err := fs.SaveFile(dir, filename, resp.Body)
			resp.Body.Close()
			if err != nil {
				results = append(results, map[string]any{"url": it.url, "success": false, "error": err.Error()})
				continue
			}
			results = append(results, map[string]any{
				"url": it.url, "success": true,
				"path": filepath.ToSlash(savedPath), "filename": filename,
			})
		}
		return ToJSON(map[string]any{"message": fmt.Sprintf("已处理 %d 项", len(items)), "results": results})
	},
})

// ── 增删改查移 ──────────────────────────────────

r.Register(&base.ToolDef{
	Name:        "file_copy",
	Description: "复制沙箱中的文件或目录。支持批量: 传 items: [{src, dst}] 一次复制多个。",
	Parameters: base.ToolParams{
		Type: "object",
		Properties: map[string]base.ParamProp{
			"src":   {Type: "string", Description: "单个源路径"},
			"dst":   {Type: "string", Description: "单个目标路径"},
			"items": {Type: "array", Description: "批量: [{src, dst}]，优先于 src/dst"},
		},
	},
	Handler: func(ctx context.Context, args map[string]any) (string, error) {
		type pair struct{ src, dst string }
		var pairs []pair
		if arr, ok := args["items"].([]any); ok {
			for _, v := range arr {
				if m, ok := v.(map[string]any); ok {
					pairs = append(pairs, pair{m["src"].(string), m["dst"].(string)})
				}
			}
		} else {
			src, _ := args["src"].(string)
			dst, _ := args["dst"].(string)
			if src != "" && dst != "" {
				pairs = []pair{{src, dst}}
			}
		}
		if len(pairs) == 0 {
			return "", fmt.Errorf("请提供 src/dst 或 items")
		}
		var results []map[string]any
		for _, p := range pairs {
			err := fs.CopyFile(p.src, p.dst)
			r := map[string]any{"src": p.src, "dst": p.dst, "success": err == nil}
			if err != nil { r["error"] = err.Error() }
			results = append(results, r)
		}
		return ToJSON(map[string]any{"message": fmt.Sprintf("已复制 %d 项", len(pairs)), "results": results})
	},
})

r.Register(&base.ToolDef{
	Name:        "file_move",
	Description: "移动文件或目录。支持批量: 传 items: [{src, dst}] 一次移动多个。",
	Parameters: base.ToolParams{
		Type: "object",
		Properties: map[string]base.ParamProp{
			"src":   {Type: "string", Description: "单个源路径"},
			"dst":   {Type: "string", Description: "单个目标路径"},
			"items": {Type: "array", Description: "批量: [{src, dst}]，优先于 src/dst"},
		},
	},
	Handler: func(ctx context.Context, args map[string]any) (string, error) {
		type pair struct{ src, dst string }
		var pairs []pair
		if arr, ok := args["items"].([]any); ok {
			for _, v := range arr {
				if m, ok := v.(map[string]any); ok {
					pairs = append(pairs, pair{m["src"].(string), m["dst"].(string)})
				}
			}
		} else {
			src, _ := args["src"].(string)
			dst, _ := args["dst"].(string)
			if src != "" && dst != "" {
				pairs = []pair{{src, dst}}
			}
		}
		if len(pairs) == 0 {
			return "", fmt.Errorf("请提供 src/dst 或 items")
		}
		var results []map[string]any
		for _, p := range pairs {
			err := fs.Rename(p.src, p.dst)
			r := map[string]any{"src": p.src, "dst": p.dst, "success": err == nil}
			if err != nil { r["error"] = err.Error() }
			results = append(results, r)
		}
		return ToJSON(map[string]any{"message": fmt.Sprintf("已移动 %d 项", len(pairs)), "results": results})
	},
})

r.Register(&base.ToolDef{
	Name:        "file_rename",
	Description: "重命名沙箱中的文件或目录。当用户说'把xxx改名为yyy'时调用。",
	Parameters: base.ToolParams{
		Type: "object",
		Properties: map[string]base.ParamProp{
			"path": {Type: "string", Description: "当前完整路径"},
			"name": {Type: "string", Description: "新名称 (仅文件名)"},
		},
		Required: []string{"path", "name"},
	},
	Handler: func(ctx context.Context, args map[string]any) (string, error) {
		path, _ := args["path"].(string)
		name, _ := args["name"].(string)
		if path == "" || name == "" {
			return "", fmt.Errorf("path 和 name 不能为空")
		}
		dir := "/"
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			dir = path[:idx]
			if dir == "" {
				dir = "/"
			}
		}
		newPath := dir + "/" + name
		if err := fs.Rename(path, newPath); err != nil {
			return "", err
		}
		return fmt.Sprintf("已重命名: %s -> %s", path, newPath), nil
	},
})

r.Register(&base.ToolDef{
	Name:        "file_info",
	Description: "获取文件或目录的详细信息。支持批量: 传 paths 数组一次查询多个。",
	Parameters: base.ToolParams{
		Type: "object",
		Properties: map[string]base.ParamProp{
			"path":  {Type: "string", Description: "单个文件或目录路径"},
			"paths": {Type: "array", Description: "批量路径数组，优先于 path"},
		},
	},
	Handler: func(ctx context.Context, args map[string]any) (string, error) {
		var paths []string
		if arr, ok := args["paths"].([]any); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					paths = append(paths, s)
				}
			}
		} else if p, _ := args["path"].(string); p != "" {
			paths = []string{p}
		}
		if len(paths) == 0 {
			return "", fmt.Errorf("请提供 path 或 paths")
		}
		var results []map[string]any
		for _, path := range paths {
			dir := "/"
			filename := path
			if idx := strings.LastIndex(path, "/"); idx >= 0 {
				dir = path[:idx]
				if dir == "" { dir = "/" }
				filename = path[idx+1:]
			}
			files, err := fs.ListDir(dir)
			found := false
			if err == nil {
				for _, f := range files {
					if f.Name == filename {
						results = append(results, map[string]any{
							"name": f.Name, "path": f.Path, "size": f.Size,
							"is_dir": f.IsDir, "mod_time": f.ModTime,
						})
						found = true
						break
					}
				}
			}
			if !found {
				r, stat, err := fs.OpenFile(path)
				if err == nil {
					r.Close()
					results = append(results, map[string]any{
						"name": filepath.Base(path), "path": path,
						"size": stat.Size(), "is_dir": false, "mod_time": stat.ModTime(),
					})
				} else {
					results = append(results, map[string]any{"path": path, "error": "不存在"})
				}
			}
		}
		return ToJSON(map[string]any{"count": len(results), "results": results})
	},
})
}

// ── 辅助函数 ──────────────────────────────────────────

func isPrintable(s string) bool {
	nonPrintable := 0
	for _, r := range s {
		if r == 0 || (r < 32 && r != '\n' && r != '\r' && r != '\t') {
			nonPrintable++
		}
	}
	return nonPrintable < len(s)/10 // 少于10%的非打印字符视为文本
}

func filepathToSlash(p string) string {
	return filepath.ToSlash(p)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}



