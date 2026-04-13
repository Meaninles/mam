import sys
import time
from dataclasses import dataclass

from pywinauto import Application


WINDOW_TITLE = "统一文件管理系统"


@dataclass
class SmokeResult:
    file_center_upload_dialog: bool = False
    file_center_download_dialog: bool = False
    task_center_stage_visible: bool = False
    settings_dependency_services_visible: bool = False


def connect_window():
    app = Application(backend="uia").connect(title=WINDOW_TITLE)
    win = app.window(title=WINDOW_TITLE)
    win.set_focus()
    return win


def click_button(win, title, found_index=0):
    btn = win.child_window(title=title, control_type="Button", found_index=found_index)
    if not btn.exists(timeout=5):
        raise RuntimeError(f"未找到按钮: {title}")
    try:
        btn.invoke()
    except Exception:
        btn.click_input()


def has_text(win, title, control_type=None, timeout=5.0):
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            criteria = {"title": title}
            if control_type is not None:
                criteria["control_type"] = control_type
            if win.child_window(**criteria).exists(timeout=0.3):
                return True
        except Exception:
            pass
        texts = []
        try:
            texts = [ctrl.window_text() for ctrl in win.descendants() if ctrl.window_text()]
        except Exception:
            texts = []
        if title in texts:
            return True
        time.sleep(0.2)
    return False


def open_file_center(win):
    click_button(win, "文件中心")
    if not has_text(win, "刷新索引", control_type="Button", timeout=6):
        raise RuntimeError("文件中心未成功打开")


def open_task_center(win):
    click_button(win, "任务中心")
    if not has_text(win, "传输任务", control_type="Button", timeout=6):
        raise RuntimeError("任务中心未成功打开")


def open_settings_dependency_services(win):
    click_button(win, "设置")
    if not has_text(win, "依赖服务", control_type="Button", timeout=6):
        raise RuntimeError("设置页未成功打开")
    click_button(win, "依赖服务")
    if not has_text(win, "CloudDrive2 状态：在线", timeout=6):
        raise RuntimeError("依赖服务页未显示 CloudDrive2 在线状态")
    if not has_text(win, "aria2 状态：在线", timeout=6):
        raise RuntimeError("依赖服务页未显示 aria2 在线状态")


def open_sync_dialog(win, endpoint_title, found_index=0):
    click_button(win, endpoint_title, found_index=found_index)
    if not has_text(win, "确认同步", timeout=6):
        raise RuntimeError(f"点击 {endpoint_title} 后未出现确认同步对话框")


def cancel_sync_dialog(win):
    click_button(win, "取消")
    time.sleep(0.8)


def confirm_sync_dialog(win):
    buttons = [ctrl for ctrl in win.descendants(control_type="Button") if ctrl.window_text() == "确认同步"]
    if not buttons:
        raise RuntimeError("未找到确认同步按钮")
    try:
        buttons[-1].invoke()
    except Exception:
        buttons[-1].click_input()
    time.sleep(2.0)


def verify_task_center_stage(win):
    open_task_center(win)
    texts = [ctrl.window_text() for ctrl in win.descendants() if ctrl.window_text()]
    if "下载" not in texts and "上传" not in texts:
        raise RuntimeError("任务中心未看到上传/下载任务类型")
    if "当前阶段：" not in texts:
        raise RuntimeError("任务中心未显示阶段信息")


def run_smoke():
    result = SmokeResult()
    win = connect_window()

    open_file_center(win)

    # 上传发起路径：点击第一个未同步到 115 的端点，出现确认同步对话框后取消。
    open_sync_dialog(win, "newshit_115 未同步", found_index=0)
    result.file_center_upload_dialog = True
    cancel_sync_dialog(win)

    # 下载发起路径：点击第一个未同步到 NAS 的端点，确认后到任务中心检查阶段。
    open_sync_dialog(win, "zongjie_nas 未同步", found_index=0)
    result.file_center_download_dialog = True
    confirm_sync_dialog(win)

    verify_task_center_stage(win)
    result.task_center_stage_visible = True

    open_settings_dependency_services(win)
    result.settings_dependency_services_visible = True

    return result


def main():
    try:
        result = run_smoke()
    except Exception as exc:
        print(f"[FAIL] {exc}")
        sys.exit(1)

    print("[PASS] Tauri 客户端烟测完成")
    print(f"  - 文件中心上传发起对话框: {result.file_center_upload_dialog}")
    print(f"  - 文件中心下载发起对话框: {result.file_center_download_dialog}")
    print(f"  - 任务中心阶段可见: {result.task_center_stage_visible}")
    print(f"  - 设置页依赖服务状态可见: {result.settings_dependency_services_visible}")


if __name__ == "__main__":
    main()
