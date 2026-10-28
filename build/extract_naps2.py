#!/usr/bin/env python3
import sys
import os
import shutil
import zipfile

def extract_naps2(zip_path, output_dir):
    temp_dir = "naps2_temp_extract"
    os.makedirs(temp_dir, exist_ok=True)
    
    with zipfile.ZipFile(zip_path, "r") as zf:
        for info in zf.infolist():
            clean_name = info.filename.replace("\\", "/").lstrip("/")
            target = os.path.join(temp_dir, clean_name)
            if info.is_dir() or clean_name.endswith("/"):
                os.makedirs(target, exist_ok=True)
            else:
                os.makedirs(os.path.dirname(target), exist_ok=True)
                with zf.open(info) as src, open(target, "wb") as dst:
                    dst.write(src.read())

    console_dir = None
    for root, dirs, files in os.walk(temp_dir):
        if "NAPS2.Console.exe" in files:
            console_dir = root
            break

    if not console_dir:
        shutil.rmtree(temp_dir, ignore_errors=True)
        raise RuntimeError("NAPS2.Console.exe not found in extracted archive!")

    os.makedirs(output_dir, exist_ok=True)
    for item in os.listdir(console_dir):
        s = os.path.join(console_dir, item)
        d = os.path.join(output_dir, item)
        if os.path.isdir(s):
            shutil.copytree(s, d, dirs_exist_ok=True)
        else:
            shutil.copy2(s, d)

    shutil.rmtree(temp_dir, ignore_errors=True)
    print(f"NAPS2 successfully unpacked to {output_dir}")

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: extract_naps2.py <zip_path> <output_dir>")
        sys.exit(1)
    extract_naps2(sys.argv[1], sys.argv[2])
