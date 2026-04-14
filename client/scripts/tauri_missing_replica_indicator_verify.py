import sys
import time

from pywinauto import Application


WINDOW_TITLE = "统一文件管理系统"


def connect_window():
    app = Application(backend="uia").connect(title=WINDOW_TITLE)
    win = app.window(title=WINDOW_TITLE)
    win.set_focus()
    return win


def click_button(win, title: str):
    btn = win.child_window(title=title, control_type="Button")
    if not btn.exists(timeout=6):
        raise RuntimeError(f"未找到按钮: {title}")
    try:
        btn.invoke()
    except Exception:
        btn.click_input()


def wait_for_text(win, text: str, timeout: float = 10.0) -> bool:
    deadline = time.time() + timeout
    while time.time() < deadline:
        texts = []
        try:
            texts = [ctrl.window_text() for ctrl in win.descendants() if ctrl.window_text()]
        except Exception:
            texts = []
        if any(text in item for item in texts):
            return True
        time.sleep(0.25)
    return False


def run_verify():
    win = connect_window()
    click_button(win, "文件中心")

    if not wait_for_text(win, "hell", timeout=12):
        raise RuntimeError("文件中心未显示 hell 条目，无法验证副本异常标识")
    if not wait_for_text(win, "异常：未找到副本", timeout=12):
        raise RuntimeError("文件中心未显示“异常：未找到副本”红灯提示")

    print("[PASS] Tauri 文件中心副本缺失红灯提示验证通过")
    print("  - 已进入：文件中心")
    print("  - 已检测到条目：hell")
    print("  - 已检测到提示：异常：未找到副本")


def main():
    try:
        run_verify()
    except Exception as exc:
        print(f"[FAIL] {exc}")
        sys.exit(1)


if __name__ == "__main__":
    main()

