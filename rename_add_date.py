"""
批量重命名脚本：在视频文件夹名称前加上收藏日期
支持递归扫描子目录，自动找到包含 详情.txt 的文件夹进行重命名
用法: python rename_add_date.py <目标目录>
"""

import os
import re
import sys


def extract_date_from_txt(txt_path: str) -> str | None:
    """从详情.txt中提取收藏时间的日期部分 (YYYY-MM-DD)"""
    try:
        with open(txt_path, "r", encoding="utf-8") as f:
            for line in f:
                m = re.match(r"^收藏时间:\s*(\d{4}-\d{2}-\d{2})", line)
                if m:
                    return m.group(1)
    except FileNotFoundError:
        pass
    return None


def scan_dir(target_dir: str):
    """递归扫描目录，找出所有包含 详情.txt 的视频文件夹"""
    date_prefix_pattern = re.compile(r"^\d{4}-\d{2}-\d{2} ")

    renames = []
    skipped_has_prefix = 0
    skipped_no_date = 0
    total_scanned = 0

    # 递归遍历所有子目录
    for root, dirs, _files in os.walk(target_dir):
        for name in sorted(dirs):
            folder_path = os.path.join(root, name)
            txt_path = os.path.join(folder_path, "详情.txt")

            # 只处理包含 详情.txt 的文件夹（视频文件夹）
            if not os.path.exists(txt_path):
                continue

            total_scanned += 1

            # 跳过已经带日期前缀的
            if date_prefix_pattern.match(name):
                skipped_has_prefix += 1
                continue

            date_str = extract_date_from_txt(txt_path)
            if date_str is None:
                skipped_no_date += 1
                continue

            new_name = f"{date_str} {name}"
            new_path = os.path.join(root, new_name)

            # 如果目标名称已存在，跳过避免覆盖
            if os.path.exists(new_path):
                print(f"跳过（目标已存在）: {name}")
                continue

            # 相对路径用于显示
            rel_old = os.path.relpath(folder_path, target_dir)
            rel_new = os.path.relpath(new_path, target_dir)
            renames.append((name, new_name, folder_path, new_path, rel_old, rel_new))

    return renames, skipped_has_prefix, skipped_no_date, total_scanned


def main():
    # 获取目标目录
    if len(sys.argv) > 1:
        target_dir = sys.argv[1]
    else:
        print("用法: python rename_add_date.py <目标目录>")
        print("示例: python rename_add_date.py D:\\B站收藏\\downloads")
        sys.exit(1)

    if not os.path.isdir(target_dir):
        print(f"错误: 目录不存在 - {target_dir}")
        sys.exit(1)

    # 扫描
    renames, skipped_has_prefix, skipped_no_date, total_scanned = scan_dir(target_dir)

    # 打印统计
    print(f"扫描完成: 共发现 {total_scanned} 个视频文件夹")
    print(f"  待重命名: {len(renames)} 个")
    if skipped_has_prefix:
        print(f"  跳过（已有日期前缀）: {skipped_has_prefix} 个")
    if skipped_no_date:
        print(f"  跳过（详情.txt中无收藏时间）: {skipped_no_date} 个")

    if not renames:
        print("\n没有需要重命名的文件夹")
        return

    # 打印预览（最多显示20个）
    preview_count = min(len(renames), 20)
    print(f"\n即将执行以下重命名（显示前 {preview_count} 个，共 {len(renames)} 个）:")
    print("-" * 70)
    for i in range(preview_count):
        _old, _new, _old_path, _new_path, rel_old, rel_new = renames[i]
        print(f"  {rel_old}")
        print(f"  → {rel_new}")
    if len(renames) > preview_count:
        print(f"  ... 还有 {len(renames) - preview_count} 个")

    # 确认（--yes 跳过确认直接执行）
    if "--yes" in sys.argv:
        print(f"(--yes 模式，跳过确认)")
    else:
        print()
        confirm = input(f"确认执行以上 {len(renames)} 个重命名？(y/n): ").strip().lower()
        if confirm != "y":
            print("已取消")
            return

    # 执行重命名
    success = 0
    fail = 0
    for old_name, new_name, old_path, new_path, _rel_old, _rel_new in renames:
        try:
            os.rename(old_path, new_path)
            print(f"  OK: {old_name} -> {new_name}")
            success += 1
        except OSError as e:
            print(f"  失败: {old_name} - {e}")
            fail += 1

    print(f"\n完成! 成功: {success}, 失败: {fail}")


if __name__ == "__main__":
    main()
