"""PyInstaller one-file spec for musipal.

Build with:
  .venv/bin/python -m PyInstaller packaging/pyinstaller-onefile.spec

Note: This still depends on system VLC/libVLC at runtime.
"""

import os

block_cipher = None

entry_script = os.path.abspath(os.path.join(SPECPATH, '..', 'src', 'musipal', 'cli.py'))

a = Analysis(
    [entry_script],
    pathex=[os.path.abspath(os.path.join(SPECPATH, '..'))],
    binaries=[],
    datas=[],
    hiddenimports=[],
    hookspath=[],
    hooksconfig={},
    runtime_hooks=[],
    excludes=[],
    win_no_prefer_redirects=False,
    win_private_assemblies=False,
    cipher=block_cipher,
    noarchive=False,
)

pyz = PYZ(a.pure, a.zipped_data, cipher=block_cipher)

# One-file executable: do NOT use COLLECT.
exe = EXE(
    pyz,
    a.scripts,
    a.binaries,
    a.zipfiles,
    a.datas,
    [],
    name='musipal',
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=True,
    console=True,
    disable_windowed_traceback=False,
)
