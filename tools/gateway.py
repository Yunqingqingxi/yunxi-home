"""
Yunxi Home - AI Tool Gateway
=============================
FastAPI server that exposes all home server tools as REST endpoints.
Registered with Open WebUI as a tool plugin.
Each tool wraps the corresponding service's API.
"""

import os
import json
import time
import hashlib
import hmac
import httpx
from typing import Optional
from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel

app = FastAPI(title="Yunxi Home Tools", version="4.0.0")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

# ── Configuration ──────────────────────────────────────────

class Config:
    YUNXI_HOME_URL = os.getenv("YUNXI_HOME_URL", "http://yunxi-home:9981")
    YUNXI_TOKEN = os.getenv("YUNXI_TOKEN", "")
    NEXTCLOUD_URL = os.getenv("NEXTCLOUD_URL", "http://nextcloud")
    NEXTCLOUD_USER = os.getenv("NEXTCLOUD_USER", "admin")
    NEXTCLOUD_PASS = os.getenv("NEXTCLOUD_PASS", "")
    JELLYFIN_URL = os.getenv("JELLYFIN_URL", "http://jellyfin:8096")
    JELLYFIN_KEY = os.getenv("JELLYFIN_API_KEY", "")
    ARIA2_URL = os.getenv("ARIA2_URL", "http://aria2:6800/jsonrpc")
    ARIA2_SECRET = os.getenv("ARIA2_RPC_SECRET", "")
    HA_URL = os.getenv("HA_URL", "http://homeassistant:8123")
    HA_TOKEN = os.getenv("HA_TOKEN", "")
    APPRISE_URL = os.getenv("APPRISE_URL", "http://apprise:8000")
    WG_URL = os.getenv("WG_URL", "http://wireguard:51821")


    # FileBrowser sandbox
    FILEBROWSER_URL = os.getenv("FILEBROWSER_URL", "http://filebrowser:8080")
    FILEBROWSER_USER = os.getenv("FILEBROWSER_USER", "ai-tool")
    FILEBROWSER_PASS = os.getenv("FILEBROWSER_PASS", "")

config = Config()


# ── HTTP Client ────────────────────────────────────────────

client = httpx.AsyncClient(timeout=30.0)

async def call_yunxi(method: str, path: str, **kwargs) -> dict:
    headers = {"Authorization": f"Bearer {config.YUNXI_TOKEN}"} if config.YUNXI_TOKEN else {}
    r = await client.request(method, f"{config.YUNXI_HOME_URL}{path}", headers=headers, **kwargs)
    return r.json()

# ── Tool API Models ────────────────────────────────────────

class ToolCall(BaseModel):
    name: str
    arguments: dict = {}

class ToolResult(BaseModel):
    success: bool
    data: Optional[dict] = None
    error: Optional[str] = None

# ── NAS Tools ──────────────────────────────────────────────

async def nas_list_files(path: str = "/") -> ToolResult:
    """List files in a directory via yunxi-home NAS API."""
    try:
        r = await call_yunxi("GET", f"/api/nas/files?path={path}")
        return ToolResult(success=True, data=r.get("data"))
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def nas_search(query: str, path: str = "/") -> ToolResult:
    """Search files via Nextcloud WebDAV or yunxi-home."""
    try:
        r = await call_yunxi("GET", f"/api/nas/search?q={query}&path={path}")
        return ToolResult(success=True, data=r.get("data"))
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def nas_share(file_path: str, expire_days: int = 7, password: str = "") -> ToolResult:
    """Generate a share link."""
    try:
        r = await call_yunxi("POST", "/api/nas/shares", json={
            "file_path": file_path,
            "expire_days": expire_days,
            "password": password or None,
        })
        return ToolResult(success=True, data=r.get("data"))
    except Exception as e:
        return ToolResult(success=False, error=str(e))

# ── Aria2 Tools ────────────────────────────────────────────

async def _aria2_rpc(method: str, params: list = None) -> dict:
    payload = {
        "jsonrpc": "2.0",
        "id": "yunxi",
        "method": method,
        "params": [f"token:{config.ARIA2_SECRET}"] + (params or []),
    }
    r = await client.post(config.ARIA2_URL, json=payload)
    return r.json()

async def aria2_add_uri(uri: str, save_path: str = "/downloads") -> ToolResult:
    try:
        result = await _aria2_rpc("aria2.addUri", [[uri], {"dir": save_path}])
        return ToolResult(success=True, data={"gid": result.get("result")})
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def aria2_list_active() -> ToolResult:
    try:
        result = await _aria2_rpc("aria2.tellActive")
        tasks = []
        for item in result.get("result", []):
            tasks.append({
                "gid": item.get("gid"),
                "name": item.get("bittorrent", {}).get("info", {}).get("name") or item.get("files", [{}])[0].get("path", "Unknown"),
                "progress": f"{int(item.get('completedLength', 0) / max(item.get('totalLength', 1), 1) * 100)}%",
                "speed": f"{item.get('downloadSpeed', 0) / 1024 / 1024:.1f} MB/s",
                "status": item.get("status"),
            })
        return ToolResult(success=True, data={"tasks": tasks, "count": len(tasks)})
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def aria2_pause_all() -> ToolResult:
    try:
        await _aria2_rpc("aria2.pauseAll")
        return ToolResult(success=True, data={"message": "所有任务已暂停"})
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def aria2_resume_all() -> ToolResult:
    try:
        await _aria2_rpc("aria2.unpauseAll")
        return ToolResult(success=True, data={"message": "所有任务已恢复"})
    except Exception as e:
        return ToolResult(success=False, error=str(e))

# ── Jellyfin Tools ─────────────────────────────────────────

async def jellyfin_search(term: str, media_type: str = None) -> ToolResult:
    try:
        headers = {"X-Emby-Token": config.JELLYFIN_KEY}
        params = {"SearchTerm": term, "IncludeItemTypes": media_type or "Movie,Series,MusicAlbum"}
        r = await client.get(f"{config.JELLYFIN_URL}/Search/Hints", headers=headers, params=params)
        items = r.json().get("SearchHints", [])
        results = [{"name": i["Name"], "year": i.get("ProductionYear"), "type": i.get("Type")} for i in items[:10]]
        return ToolResult(success=True, data={"results": results, "count": len(results)})
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def jellyfin_scan_library() -> ToolResult:
    try:
        headers = {"X-Emby-Token": config.JELLYFIN_KEY}
        r = await client.post(f"{config.JELLYFIN_URL}/Library/Refresh", headers=headers)
        return ToolResult(success=True, data={"message": "媒体库扫描已触发"})
    except Exception as e:
        return ToolResult(success=False, error=str(e))

# ── WireGuard Tools ────────────────────────────────────────

async def vpn_add_client(name: str) -> ToolResult:
    try:
        r = await client.post(f"{config.WG_URL}/api/clients", json={"name": name})
        return ToolResult(success=True, data=r.json())
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def vpn_list_clients() -> ToolResult:
    try:
        r = await client.get(f"{config.WG_URL}/api/clients")
        return ToolResult(success=True, data=r.json())
    except Exception as e:
        return ToolResult(success=False, error=str(e))

# ── Home Assistant Tools ───────────────────────────────────

async def ha_get_state(entity_id: str) -> ToolResult:
    try:
        headers = {"Authorization": f"Bearer {config.HA_TOKEN}"}
        r = await client.get(f"{config.HA_URL}/api/states/{entity_id}", headers=headers)
        data = r.json()
        return ToolResult(success=True, data={
            "entity_id": data.get("entity_id"),
            "state": data.get("state"),
            "attributes": data.get("attributes", {}),
        })
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def ha_control(entity_id: str, action: str, brightness: int = None) -> ToolResult:
    try:
        headers = {"Authorization": f"Bearer {config.HA_TOKEN}"}
        domain = entity_id.split(".")[0]
        service = f"{action}"
        payload = {"entity_id": entity_id}
        if brightness is not None:
            payload["brightness"] = brightness
        r = await client.post(f"{config.HA_URL}/api/services/{domain}/{service}", headers=headers, json=payload)
        return ToolResult(success=True, data={"message": f"{entity_id} {action} 成功"})
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def ha_run_scene(scene_name: str) -> ToolResult:
    try:
        headers = {"Authorization": f"Bearer {config.HA_TOKEN}"}
        r = await client.get(f"{config.HA_URL}/api/states", headers=headers)
        scenes = [s for s in r.json() if s["entity_id"].startswith("scene.") and scene_name.lower() in s["entity_id"].lower()]
        if not scenes:
            return ToolResult(success=False, error=f"未找到场景: {scene_name}")
        entity_id = scenes[0]["entity_id"]
        r = await client.post(f"{config.HA_URL}/api/services/scene/turn_on", headers=headers, json={"entity_id": entity_id})
        return ToolResult(success=True, data={"message": f"场景 {entity_id} 已启动"})
    except Exception as e:
        return ToolResult(success=False, error=str(e))

# ── System Tools ───────────────────────────────────────────

async def system_status() -> ToolResult:
    try:
        r = await call_yunxi("GET", "/api/status")
        data = r.get("data", {})
        return ToolResult(success=True, data={
            "hostname": data.get("system", {}).get("hostname"),
            "cpu_usage": f"{data.get('system', {}).get('cpu_usage', 0):.1f}%",
            "mem_usage": f"{data.get('system', {}).get('mem_usage', 0):.1f}%",
            "uptime": data.get("uptime"),
            "goroutines": data.get("goroutines"),
        })
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def container_status() -> ToolResult:
    try:
        r = await call_yunxi("GET", "/api/docker/containers")
        data = r.get("data", [])
        containers = [{"name": c["name"], "state": c["state"], "status": c["status"]} for c in data]
        return ToolResult(success=True, data={"containers": containers, "count": len(containers)})
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def container_restart(container_name: str) -> ToolResult:
    try:
        r = await call_yunxi("POST", f"/api/docker/containers/{container_name}/restart")
        return ToolResult(success=True, data={"message": f"容器 {container_name} 已重启"})
    except Exception as e:
        return ToolResult(success=False, error=str(e))

# ── Notification Tools ─────────────────────────────────────

async def send_notification(title: str, message: str, target: str = "all") -> ToolResult:
    try:
        r = await client.post(f"{config.APPRISE_URL}/notify/{target}", json={
            "title": title,
            "body": message,
        })
        return ToolResult(success=True, data={"message": "通知已发送"})
    except Exception as e:
        return ToolResult(success=False, error=str(e))

# ── DNS Tools ──────────────────────────────────────────────

async def dns_list_domains() -> ToolResult:
    try:
        r = await call_yunxi("GET", "/api/domains")
        return ToolResult(success=True, data=r.get("data"))
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def dns_trigger_update() -> ToolResult:
    try:
        r = await call_yunxi("POST", "/api/trigger")
        return ToolResult(success=True, data=r.get("data"))
    except Exception as e:
        return ToolResult(success=False, error=str(e))


# ── FileBrowser Sandbox API ────────────────────────────────

_fb_token = None
_fb_token_expiry = 0

async def _fb_auth() -> str:
    """Get or refresh FileBrowser JWT token."""
    global _fb_token, _fb_token_expiry
    if _fb_token and time.time() < _fb_token_expiry - 60:
        return _fb_token
    r = await client.post(f"{config.FILEBROWSER_URL}/api/login", json={
        "username": config.FILEBROWSER_USER,
        "password": config.FILEBROWSER_PASS,
    })
    r.raise_for_status()
    _fb_token = r.text.strip('"')
    _fb_token_expiry = time.time() + 3600
    return _fb_token

async def _fb_request(method: str, path: str, **kwargs) -> dict:
    """Make an authenticated FileBrowser API request."""
    token = await _fb_auth()
    headers = kwargs.pop("headers", {})
    headers["X-Auth"] = token
    r = await client.request(method, f"{config.FILEBROWSER_URL}/api{path}", headers=headers, **kwargs)
    if r.status_code >= 400:
        return {"error": r.text, "status": r.status_code}
    if r.headers.get("content-type", "").startswith("application/json"):
        return r.json()
    return {"data": r.text}

# ── FileBrowser Sandbox Tools ──────────────────────────────

async def sandbox_list(path: str = "/") -> ToolResult:
    """List files in sandbox directory via FileBrowser API."""
    try:
        data = await _fb_request("GET", f"/resources{path}")
        if isinstance(data, list):
            items = data
            if path != "/":
                items = data.get("items", data)
        elif isinstance(data, dict):
            items = data.get("items", data)
        else:
            items = []

        files = []
        for item in (items if isinstance(items, list) else [items]):
            files.append({
                "name": item.get("name"),
                "path": item.get("path"),
                "size": item.get("size"),
                "is_dir": item.get("isDir", item.get("is_dir", False)),
                "modified": item.get("modified"),
                "ext": item.get("extension", ""),
            })
        return ToolResult(success=True, data={"path": path, "files": files, "count": len(files)})
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def sandbox_mkdir(path: str) -> ToolResult:
    """Create a directory in sandbox via FileBrowser API."""
    try:
        data = await _fb_request("POST", "/resources" + path + "/?override=false")
        return ToolResult(success=True, data={"message": f"目录已创建: {path}"})
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def sandbox_delete(path: str, confirm: bool = False) -> ToolResult:
    """Delete file/dir in sandbox. Requires confirm=True."""
    try:
        if not confirm:
            # Preview
            data = await _fb_request("GET", f"/resources{path}")
            items = data if isinstance(data, list) else data.get("items", [data])
            return ToolResult(success=True, data={
                "warning": f"即将删除 {path}，请设置 confirm=true 确认。",
                "preview": items if isinstance(items, list) else [items],
                "action": "delete",
            })
        await _fb_request("DELETE", f"/resources{path}")
        return ToolResult(success=True, data={"message": f"已删除: {path}"})
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def sandbox_read(path: str, base64: bool = False) -> ToolResult:
    """Read a file from sandbox."""
    try:
        r = await client.request("GET", f"{config.FILEBROWSER_URL}/api/raw{path}",
                                 headers={"X-Auth": await _fb_auth()})
        if r.status_code >= 400:
            return ToolResult(success=False, error=f"读取失败: {r.status_code}")
        content = r.text
        if base64:
            import base64 as b64
            content = b64.b64encode(r.content).decode()
        # Truncate large files
        if len(content) > 10000:
            content = content[:10000] + f"
... (截断，共 {len(r.content)} 字节)"
        return ToolResult(success=True, data={"path": path, "content": content, "size": len(r.content)})
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def sandbox_write(path: str, content: str = "", base64: str = "", confirm: bool = False) -> ToolResult:
    """Write a file to sandbox. Overwrite requires confirm=True."""
    try:
        # Check if exists
        try:
            await _fb_request("GET", f"/resources{path}")
            exists = True
        except:
            exists = False

        if exists and not confirm:
            return ToolResult(success=True, data={
                "warning": f"文件 {path} 已存在，请设置 confirm=true 确认覆盖。",
                "action": "write",
            })

        if base64:
            import base64 as b64
            data = b64.b64decode(base64)
        else:
            data = content.encode("utf-8")

        # Upload via FileBrowser API
        from io import BytesIO
        token = await _fb_auth()
        files = {"file": (path.split("/")[-1], BytesIO(data))}
        # Determine parent directory
        parent = "/".join(path.split("/")[:-1]) or "/"
        r = await client.post(
            f"{config.FILEBROWSER_URL}/api/resources{parent}",
            headers={"X-Auth": token},
            files={"file": (path.split("/")[-1], BytesIO(data))},
            data={"override": "true"},
        )
        if r.status_code >= 400:
            return ToolResult(success=False, error=f"写入失败: {r.text}")
        return ToolResult(success=True, data={"message": f"文件已写入: {path}"})
    except Exception as e:
        return ToolResult(success=False, error=str(e))

async def sandbox_diskinfo() -> ToolResult:
    """Get sandbox disk usage via FileBrowser."""
    try:
        data = await _fb_request("GET", "/usage")
        return ToolResult(success=True, data={
            "total_bytes": data.get("total", 0),
            "used_bytes": data.get("used", 0),
            "free_bytes": data.get("free", 0),
        })
    except Exception as e:
        return ToolResult(success=False, error=str(e))

# ── Tool Router ────────────────────────────────────────────

TOOL_MAP = {
    "nas_list_files": nas_list_files,
    "nas_search": nas_search,
    "nas_share": nas_share,
    "aria2_add_uri": aria2_add_uri,
    "aria2_list_active": aria2_list_active,
    "aria2_pause_all": aria2_pause_all,
    "aria2_resume_all": aria2_resume_all,
    "jellyfin_search": jellyfin_search,
    "jellyfin_scan_library": jellyfin_scan_library,
    "vpn_add_client": vpn_add_client,
    "vpn_list_clients": vpn_list_clients,
    "ha_get_state": ha_get_state,
    "ha_control": ha_control,
    "ha_run_scene": ha_run_scene,
    "system_status": system_status,
    "container_status": container_status,
    "container_restart": container_restart,
    "send_notification": send_notification,
    "dns_list_domains": dns_list_domains,
    "dns_trigger_update": dns_trigger_update,
    "sandbox_list": sandbox_list,
    "sandbox_mkdir": sandbox_mkdir,
    "sandbox_delete": sandbox_delete,
    "sandbox_read": sandbox_read,
    "sandbox_write": sandbox_write,
    "sandbox_diskinfo": sandbox_diskinfo,
}

# ── API Endpoints ──────────────────────────────────────────

@app.get("/health")
async def health():
    return {"status": "ok", "tools": len(TOOL_MAP)}

@app.post("/tools/{tool_name}")
async def execute_tool(tool_name: str, call: ToolCall):
    """Execute a single tool. Called by Open WebUI tool plugin."""
    if tool_name not in TOOL_MAP:
        raise HTTPException(404, f"Unknown tool: {tool_name}")
    handler = TOOL_MAP[tool_name]
    result = await handler(**call.arguments)
    return result.model_dump()

@app.post("/tools")
async def execute_tools(calls: list[ToolCall]):
    """Execute multiple tools in sequence."""
    results = {}
    for call in calls:
        if call.name not in TOOL_MAP:
            results[call.name] = ToolResult(success=False, error=f"Unknown tool: {call.name}").model_dump()
        else:
            handler = TOOL_MAP[call.name]
            results[call.name] = (await handler(**call.arguments)).model_dump()
    return results

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=9000)
