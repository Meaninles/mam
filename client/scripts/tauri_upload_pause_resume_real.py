import json
import os
import random
import string
import sys
import time
import ctypes
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Dict, List, Optional
from urllib.parse import urlencode
from urllib.request import Request, urlopen

from pywinauto import Application, Desktop
from pywinauto.keyboard import send_keys


WINDOW_TITLE = "统一文件管理系统"
CENTER_BASE_URL = "http://127.0.0.1:8080"
TARGET_LIBRARY_NAME = "测试库2-临时-只有网盘"
FILE_COUNT = 16
FILE_SIZE_MB_SEQUENCE = [10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 12, 14, 16, 18, 15]
PAUSE_RESUME_SCHEDULE = [0.6, 2.2, 1.0, 3.0, 0.8, 4.0]
UPLOAD_TIMEOUT_SECONDS = 1800
SCAN_TIMEOUT_SECONDS = 600


@dataclass
class TestFile:
    name: str
    size_bytes: int


@dataclass
class TestContext:
    library_id: str
    mount_id: str
    folder_name: str
    folder_path: Path
    files: List[TestFile]
    job_id: Optional[str] = None


def api_json(path: str, method: str = "GET", payload: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
    body = None
    headers = {}
    if payload is not None:
        body = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    request = Request(f"{CENTER_BASE_URL}{path}", data=body, headers=headers, method=method)
    with urlopen(request, timeout=30) as response:  # nosec B310
        return json.loads(response.read().decode("utf-8"))


def connect_window():
    app = Application(backend="uia").connect(title=WINDOW_TITLE)
    win = app.window(title=WINDOW_TITLE)
    win.set_focus()
    return win


def all_buttons(win, title: str):
    results = []
    for ctrl in win.descendants(control_type="Button"):
        try:
            if ctrl.window_text() == title:
                results.append(ctrl)
        except Exception:
            pass
    return results


def click_button(win, title: str, pick: str = "first"):
    buttons = all_buttons(win, title)
    if not buttons:
        raise RuntimeError(f"未找到按钮: {title}")
    if pick == "rightmost":
        buttons.sort(key=lambda item: (item.rectangle().left, item.rectangle().top))
        target = buttons[-1]
    elif pick == "leftmost":
        buttons.sort(key=lambda item: (item.rectangle().left, item.rectangle().top))
        target = buttons[0]
    else:
        target = buttons[0]
    try:
        target.invoke()
    except Exception:
        target.click_input()
    time.sleep(0.8)


def try_click_button(win, title: str, pick: str = "first") -> bool:
    try:
        click_button(win, title, pick=pick)
        return True
    except Exception:
        return False


def click_tab_item(win, title: str) -> bool:
    candidates = []
    for ctrl in win.descendants(control_type="TabItem"):
        try:
            if ctrl.window_text() == title:
                candidates.append(ctrl)
        except Exception:
            pass
    if not candidates:
        return False
    try:
        candidates[0].select()
    except Exception:
        try:
            candidates[0].invoke()
        except Exception:
            candidates[0].click_input()
    time.sleep(0.8)
    return True


def wait_for_button(win, title: str, timeout: float = 10.0) -> bool:
    deadline = time.time() + timeout
    while time.time() < deadline:
        if all_buttons(win, title):
            return True
        time.sleep(0.2)
    return False


def set_edit(win, title: str, value: str):
    edit = win.child_window(title=title, control_type="Edit")
    if not edit.exists(timeout=5):
        raise RuntimeError(f"未找到输入框: {title}")
    wrapper = edit.wrapper_object()
    wrapper.click_input()
    send_keys("^a{BACKSPACE}")
    time.sleep(0.2)
    wrapper.type_keys(value, with_spaces=True, set_foreground=True)
    time.sleep(0.6)


def get_foreground_title() -> str:
    user32 = ctypes.windll.user32
    hwnd = user32.GetForegroundWindow()
    title = ctypes.create_unicode_buffer(512)
    user32.GetWindowTextW(hwnd, title, 512)
    return title.value


def wait_for_folder_dialog(timeout: float = 20.0):
    deadline = time.time() + timeout
    while time.time() < deadline:
        title = get_foreground_title()
        if title == "选择要上传的文件夹":
            return Desktop(backend="win32").window(title="选择要上传的文件夹", class_name="#32770")
        time.sleep(0.2)
    raise RuntimeError("未弹出“选择要上传的文件夹”对话框")


def wait_for_security_prompt(timeout: float = 20.0):
    deadline = time.time() + timeout
    while time.time() < deadline:
        for window in Desktop(backend="win32").windows():
            try:
                title = window.window_text()
            except Exception:
                continue
            if "是否将文件上传到此站点" in title:
                return window
        time.sleep(0.2)
    raise RuntimeError("未弹出上传站点安全确认框")


def submit_folder_dialog(folder_path: Path):
    dialog = wait_for_folder_dialog()
    dialog.set_focus()
    send_keys("%d")
    time.sleep(0.5)
    send_keys("^a")
    time.sleep(0.2)
    send_keys(str(folder_path).replace("\\", "{\\}"), with_spaces=True)
    time.sleep(0.5)
    send_keys("{ENTER}")
    time.sleep(1.0)
    upload_button = dialog.child_window(title="上传", class_name="Button")
    if not upload_button.exists(timeout=5):
        raise RuntimeError("目录选择框中未找到“上传”按钮")
    upload_button.click()
    time.sleep(1.0)
    time.sleep(1.5)


def confirm_site_upload_prompt(win, timeout: float = 20.0) -> bool:
    deadline = time.time() + timeout
    while time.time() < deadline:
        texts = [ctrl.window_text() for ctrl in win.descendants() if ctrl.window_text()]
        if any("上传到此站点" in text for text in texts):
            buttons = all_buttons(win, "上传")
            if not buttons:
                raise RuntimeError("站点上传确认层中未找到“上传”按钮")
            buttons.sort(key=lambda item: (item.rectangle().left, item.rectangle().top))
            target = buttons[0]
            try:
                target.invoke()
            except Exception:
                target.click_input()
            time.sleep(2.0)
            return True
        time.sleep(0.2)
    return False


def wait_for_hidden_folder_input(win, timeout: float = 15.0):
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            ctrl = win.child_window(title_re="上传文件夹选择:.*").wrapper_object()
            return ctrl
        except Exception:
            time.sleep(0.3)
    raise RuntimeError("文件中心未出现上传文件夹控件")


def dismiss_residual_dialogs(win):
    for _ in range(3):
        cleaned = False
        texts = [ctrl.window_text() for ctrl in win.descendants() if ctrl.window_text()]
        if any("上传到此站点" in text for text in texts):
            buttons = all_buttons(win, "取消")
            buttons.sort(key=lambda item: (item.rectangle().left, item.rectangle().top))
            if buttons:
                try:
                    buttons[0].invoke()
                except Exception:
                    buttons[0].click_input()
                time.sleep(1.0)
                cleaned = True
        elif "选择要上传的文件夹" in texts:
            buttons = all_buttons(win, "取消")
            buttons.sort(key=lambda item: (item.rectangle().left, item.rectangle().top))
            if buttons:
                try:
                    buttons[0].invoke()
                except Exception:
                    buttons[0].click_input()
                time.sleep(1.0)
                cleaned = True
        if not cleaned:
            break

    for window in Desktop(backend="win32").windows():
        try:
            title = window.window_text()
            cls = window.class_name()
        except Exception:
            continue
        if cls != "#32770" and "上传到此站点" not in title and "选择要上传的文件夹" not in title:
            continue
        for label in ("取消", "关闭", "否"):
            try:
                button = window.child_window(title=label, class_name="Button")
                if button.exists(timeout=0.5):
                    button.click()
                    time.sleep(1.0)
                    break
            except Exception:
                pass


def activate_file_center(win):
    dismiss_residual_dialogs(win)
    try:
        wait_for_hidden_folder_input(win, timeout=2.0)
        return
    except Exception:
        pass
    try_click_button(win, TARGET_LIBRARY_NAME, pick="leftmost")
    if not try_click_button(win, "文件中心", pick="leftmost"):
        click_tab_item(win, "文件中心")
    wait_for_hidden_folder_input(win, timeout=20.0)


def activate_task_center(win):
    try_click_button(win, "文件中心", pick="leftmost")
    time.sleep(0.8)
    if not try_click_button(win, "任务中心", pick="leftmost"):
        click_tab_item(win, "任务中心")
    if not wait_for_button(win, "传输任务", timeout=10):
        raise RuntimeError("任务中心未成功打开")
    click_button(win, "传输任务")
    click_button(win, "导入")
    deadline = time.time() + 10.0
    while time.time() < deadline:
        combos = [ctrl.window_text() for ctrl in win.descendants(control_type="ComboBox")]
        if combos and combos[0] == "任务状态":
            return
        time.sleep(0.3)
    raise RuntimeError("任务中心未成功切换到导入视图")


def make_test_folder() -> TestContext:
    libraries = api_json("/api/libraries")["data"]
    target_library = next((item for item in libraries if item["name"] == TARGET_LIBRARY_NAME), None)
    if target_library is None:
        raise RuntimeError(f"未找到资产库: {TARGET_LIBRARY_NAME}")

    mounts = api_json("/api/storage/local-folders")["data"]
    target_mount = next((item for item in mounts if item["libraryId"] == target_library["id"] and item["name"] == TARGET_LIBRARY_NAME), None)
    if target_mount is None:
        raise RuntimeError("未找到目标网盘挂载")

    suffix = time.strftime("%Y%m%d-%H%M%S")
    nonce = "".join(random.choice(string.ascii_lowercase + string.digits) for _ in range(6))
    folder_name = f"uia-pause-resume-{suffix}-{nonce}"
    folder_path = Path(os.environ["TEMP"]) / folder_name
    folder_path.mkdir(parents=True, exist_ok=True)

    names = [
        "DSC_0702.jpg",
        "IMG20251228092755.dng",
        "IMG20251228092756.heic",
        "IMG20251228092757.jpg",
        "IMG20251228092758.mov",
        "IMG20251228092759.mp4",
        "IMG20251228092800.jpg",
        "IMG20251228092801.dng",
        "IMG20251228092802.jpg",
        "IMG20251228092803.heic",
        "IMG20251228092804.jpg",
        "IMG20251228092805.mov",
        "IMG20251228092806.mp4",
        "IMG20251228092807.jpg",
        "IMG20251228092808.dng",
        "IMG20251228092809.jpg",
    ]

    files: List[TestFile] = []
    for index, size_mb in enumerate(FILE_SIZE_MB_SEQUENCE):
        name = names[index]
        path = folder_path / name
        total_bytes = size_mb * 1024 * 1024
        seed = f"{folder_name}-{name}".encode("utf-8")
        chunk = (seed * ((1024 * 1024 // len(seed)) + 1))[: 1024 * 1024]
        with path.open("wb") as handle:
            for _ in range(size_mb):
                handle.write(chunk)
        files.append(TestFile(name=name, size_bytes=total_bytes))

    return TestContext(
        library_id=target_library["id"],
        mount_id=target_mount["id"],
        folder_name=folder_name,
        folder_path=folder_path,
        files=files,
    )


def latest_file_center_import_job(library_id: str, previous_ids: set[str]) -> Optional[str]:
    query = urlencode({"libraryId": library_id, "sourceDomain": "FILE_CENTER", "page": 1, "pageSize": 20})
    payload = api_json(f"/api/jobs?{query}")["data"]
    for item in payload["items"]:
        if item["jobIntent"] == "IMPORT" and item["id"] not in previous_ids:
            return item["id"]
    return None


def wait_for_job_creation(library_id: str, previous_ids: set[str], timeout: float = 60.0) -> str:
    deadline = time.time() + timeout
    while time.time() < deadline:
        job_id = latest_file_center_import_job(library_id, previous_ids)
        if job_id:
            return job_id
        time.sleep(1.0)
    raise RuntimeError("上传后未观察到新的文件中心导入作业")


def current_job_detail(job_id: str) -> Dict[str, Any]:
    return api_json(f"/api/jobs/{job_id}")["data"]


def wait_until_task_visible(win, keyword: str, timeout: float = 60.0):
    activate_task_center(win)
    set_edit(win, "搜索任务", "")
    deadline = time.time() + timeout
    while time.time() < deadline:
        texts = [ctrl.window_text() for ctrl in win.descendants() if ctrl.window_text()]
        if any(keyword in text for text in texts):
            return
        time.sleep(0.5)
    raise RuntimeError("任务中心未显示目标上传任务")


def click_task_action(win, title: str, timeout: float = 15.0):
    deadline = time.time() + timeout
    while time.time() < deadline:
        buttons = [ctrl for ctrl in win.descendants(control_type="Button") if ctrl.window_text() == title]
        buttons.sort(key=lambda ctrl: (ctrl.rectangle().top, ctrl.rectangle().left))
        if buttons:
            try:
                buttons[0].invoke()
            except Exception:
                buttons[0].click_input()
            time.sleep(1.0)
            return
        time.sleep(0.3)
    raise RuntimeError(f"任务中心未找到动作按钮: {title}")


def exercise_pause_resume(win, job_id: str, task_keyword: str):
    wait_until_task_visible(win, task_keyword)
    for interval in PAUSE_RESUME_SCHEDULE:
        detail = current_job_detail(job_id)
        status = detail["job"]["status"]
        if status in ("COMPLETED", "FAILED", "PARTIAL_SUCCESS", "CANCELED"):
            return
        click_task_action(win, "暂停")
        time.sleep(interval)
        click_task_action(win, "继续")
        time.sleep(interval)


def wait_for_job_terminal(job_id: str, timeout: float = UPLOAD_TIMEOUT_SECONDS) -> Dict[str, Any]:
    deadline = time.time() + timeout
    last = None
    while time.time() < deadline:
        detail = current_job_detail(job_id)
        last = detail
        if detail["job"]["status"] in ("COMPLETED", "FAILED", "PARTIAL_SUCCESS", "CANCELED"):
            return detail
        time.sleep(2.0)
    raise RuntimeError(f"等待上传作业结束超时，最后状态: {last['job']['status'] if last else 'unknown'}")


def trigger_cloud_scan_and_wait(mount_id: str, timeout: float = SCAN_TIMEOUT_SECONDS):
    before = api_json(f"/api/storage/local-folders/{mount_id}/scan-history")["data"]["items"]
    previous_latest = before[0]["testedAt"] if before else ""
    response = api_json("/api/storage/local-folders/scan", method="POST", payload={"ids": [mount_id]})["data"]
    job_id = response["jobId"]
    wait_for_job_terminal(job_id, timeout=timeout)
    deadline = time.time() + timeout
    while time.time() < deadline:
        histories = api_json(f"/api/storage/local-folders/{mount_id}/scan-history")["data"]["items"]
        if histories and histories[0]["testedAt"] != previous_latest and histories[0]["status"] == "成功":
            return
        time.sleep(2.0)
    raise RuntimeError("等待云挂载扫描成功超时")


def browse_root_for_folder(context: TestContext) -> str:
    query = urlencode(
        {
            "page": 1,
            "pageSize": 100,
            "searchText": context.folder_name,
            "fileTypeFilter": "全部",
            "statusFilter": "全部",
            "sortValue": "名称",
            "sortDirection": "asc",
        }
    )
    payload = api_json(f"/api/libraries/{context.library_id}/browse?{query}")["data"]
    for item in payload["items"]:
        if item["type"] == "folder" and item["name"] == context.folder_name:
            return item["id"]
    raise RuntimeError(f"扫描后未在文件中心根目录找到上传文件夹: {context.folder_name}")


def format_size(bytes_value: int) -> str:
    kb = 1024
    mb = 1024 * kb
    gb = 1024 * mb
    if bytes_value >= gb:
        text = f"{bytes_value / gb:.1f} GB"
    elif bytes_value >= mb:
        text = f"{bytes_value / mb:.1f} MB"
    elif bytes_value >= kb:
        text = f"{bytes_value / kb:.1f} KB"
    else:
        text = f"{bytes_value} B"
    return text.replace(".0 ", " ")


def verify_remote_result(context: TestContext):
    folder_id = browse_root_for_folder(context)
    query = urlencode(
        {
            "parentId": folder_id,
            "page": 1,
            "pageSize": 200,
            "fileTypeFilter": "全部",
            "statusFilter": "全部",
            "sortValue": "名称",
            "sortDirection": "asc",
        }
    )
    payload = api_json(f"/api/libraries/{context.library_id}/browse?{query}")["data"]
    items = payload["items"]
    if len(items) != len(context.files):
        raise RuntimeError(f"远端文件数量不匹配，期望 {len(context.files)}，实际 {len(items)}")

    expected = {item.name: format_size(item.size_bytes) for item in context.files}
    actual = {item["name"]: item["size"] for item in items if item["type"] == "file"}

    missing = sorted(set(expected) - set(actual))
    extra = sorted(set(actual) - set(expected))
    if missing or extra:
        raise RuntimeError(f"远端文件名不匹配，缺失={missing}，多出={extra}")

    wrong_sizes = {name: {"expected": expected[name], "actual": actual[name]} for name in expected if actual[name] != expected[name]}
    if wrong_sizes:
        raise RuntimeError(f"远端文件大小不匹配: {wrong_sizes}")


def capture_existing_file_center_job_ids(library_id: str) -> set[str]:
    query = urlencode({"libraryId": library_id, "sourceDomain": "FILE_CENTER", "page": 1, "pageSize": 50})
    payload = api_json(f"/api/jobs?{query}")["data"]
    return {item["id"] for item in payload["items"]}


def upload_folder_via_tauri(win, context: TestContext):
    activate_file_center(win)
    existing_ids = capture_existing_file_center_job_ids(context.library_id)
    hidden_picker = wait_for_hidden_folder_input(win)
    hidden_picker.invoke()
    submit_folder_dialog(context.folder_path)
    confirm_site_upload_prompt(win)
    context.job_id = wait_for_job_creation(context.library_id, existing_ids)


def main():
    context = make_test_folder()
    print(f"[INFO] 测试目录: {context.folder_path}")
    print(f"[INFO] 目标资产库: {TARGET_LIBRARY_NAME}")
    print(f"[INFO] 目标挂载: {context.mount_id}")

    win = connect_window()
    upload_folder_via_tauri(win, context)
    print(f"[INFO] 上传作业已创建: {context.job_id}")

    exercise_pause_resume(win, context.job_id, context.folder_name)
    detail = wait_for_job_terminal(context.job_id)
    print(f"[INFO] 上传作业结束状态: {detail['job']['status']}")
    if detail["job"]["status"] != "COMPLETED":
        raise RuntimeError(f"上传作业未成功完成: {detail['job']['status']}")

    trigger_cloud_scan_and_wait(context.mount_id)
    verify_remote_result(context)

    print("[PASS] Tauri 客户端真实上传暂停/恢复测试通过")
    print(f"  - 测试文件夹: {context.folder_name}")
    print(f"  - 文件数量: {len(context.files)}")
    print(f"  - 上传作业: {context.job_id}")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"[FAIL] {exc}")
        sys.exit(1)
