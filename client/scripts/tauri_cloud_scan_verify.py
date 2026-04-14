import json
import sys
import time
from typing import Any, Dict, List
from urllib.error import URLError
from urllib.request import urlopen

from pywinauto import Application


WINDOW_TITLE = "统一文件管理系统"
CENTER_BASE_URL = "http://127.0.0.1:8080"


def get_json(url: str) -> Dict[str, Any]:
    with urlopen(url, timeout=10) as response:  # nosec B310
        payload = response.read().decode("utf-8")
    return json.loads(payload)


def connect_window():
    app = Application(backend="uia").connect(title=WINDOW_TITLE)
    win = app.window(title=WINDOW_TITLE)
    win.set_focus()
    return win


def click_button(win, title: str):
    btn = win.child_window(title=title, control_type="Button")
    if not btn.exists(timeout=5):
        raise RuntimeError(f"未找到按钮: {title}")
    try:
        btn.invoke()
    except Exception:
        btn.click_input()


def has_text(win, text: str, timeout: float = 6.0) -> bool:
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            if win.child_window(title=text).exists(timeout=0.3):
                return True
        except Exception:
            pass
        try:
            texts = [ctrl.window_text() for ctrl in win.descendants() if ctrl.window_text()]
            if text in texts:
                return True
        except Exception:
            pass
        time.sleep(0.2)
    return False


def resolve_cloud_mounts() -> List[Dict[str, Any]]:
    payload = get_json(f"{CENTER_BASE_URL}/api/storage/local-folders")
    items: List[Dict[str, Any]] = payload.get("data", [])
    cloud_mounts = [item for item in items if item.get("folderType") == "网盘"]
    if not cloud_mounts:
        raise RuntimeError("未找到网盘类型挂载，无法执行验证")
    return cloud_mounts


def load_scan_histories(mount_id: str) -> List[Dict[str, Any]]:
    payload = get_json(f"{CENTER_BASE_URL}/api/storage/local-folders/{mount_id}/scan-history")
    data = payload.get("data", {})
    return data.get("items", [])


def wait_for_new_success_history(mount_id: str, previous_count: int, timeout: float = 45.0):
    deadline = time.time() + timeout
    while time.time() < deadline:
        items = load_scan_histories(mount_id)
        if len(items) > previous_count:
            latest = items[0]
            if latest.get("status") == "成功":
                return latest, len(items)
            raise RuntimeError(f"新增扫描记录状态不是成功: {latest}")
        time.sleep(1.0)
    raise RuntimeError("等待扫描历史更新超时")


def run_verify():
    win = connect_window()
    click_button(win, "存储节点")
    if not has_text(win, "挂载管理", timeout=8):
        raise RuntimeError("未进入存储节点页面")
    click_button(win, "挂载管理")
    cloud_mounts = resolve_cloud_mounts()
    mount = None
    scan_button_text = ""
    available_buttons = [
        ctrl.window_text()
        for ctrl in win.descendants(control_type="Button")
        if ctrl.window_text() and ctrl.window_text().startswith("立即扫描 ")
    ]
    for item in cloud_mounts:
        candidate_button = f"立即扫描 {item['name']}"
        if candidate_button in available_buttons:
            mount = item
            scan_button_text = candidate_button
            break
    if mount is None:
        raise RuntimeError(
            "未在当前挂载列表中找到可扫描的网盘挂载。"
            f" 网盘挂载候选: {[item['name'] for item in cloud_mounts]}，"
            f" 当前可见扫描按钮: {available_buttons}"
        )

    mount_id = mount["id"]
    mount_name = mount["name"]
    before_histories = load_scan_histories(mount_id)
    before_count = len(before_histories)

    click_button(win, scan_button_text)

    if not has_text(win, "作业已创建", timeout=8):
        # Tauri 页面提示可能很快消失，允许继续通过后端结果验证。
        pass

    latest_history, after_count = wait_for_new_success_history(mount_id, before_count, timeout=60)
    print("[PASS] Tauri 客户端网盘挂载扫描验证通过")
    print(f"  - 挂载名称: {mount_name}")
    print(f"  - 扫描按钮: {scan_button_text}")
    print(f"  - 扫描历史: {before_count} -> {after_count}")
    print(f"  - 最新结果: {latest_history.get('status')} / {latest_history.get('summary')}")


def main():
    try:
        run_verify()
    except URLError as exc:
        print(f"[FAIL] Center 服务不可用: {exc}")
        sys.exit(1)
    except Exception as exc:
        print(f"[FAIL] {exc}")
        sys.exit(1)


if __name__ == "__main__":
    main()
