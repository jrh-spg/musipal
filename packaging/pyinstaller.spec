"""PyInstaller spec for musipal.

Build with:
    .venv/bin/python -m PyInstaller packaging/pyinstaller.spec
"""

import os

entry_script = os.path.abspath(os.path.join(SPECPATH, '..', 'src', 'musipal', 'cli.py'))

# Keep hidden imports minimal. prompt_toolkit has optional extras (e.g. SSH)
# that can pull in non-required dependencies like asyncssh.
hiddenimports = []

block_cipher = None

a = Analysis(
    [entry_script],
    pathex=[os.path.abspath(os.path.join(SPECPATH, '..'))],
    binaries=[],
    datas=[],
    hiddenimports=hiddenimports,
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

exe = EXE(
    pyz,
    a.scripts,
    [],
    exclude_binaries=True,
    name='musipal',
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=True,
    console=True,
    disable_windowed_traceback=False,
)

coll = COLLECT(
    exe,
    a.binaries,
    a.zipfiles,
    a.datas,
    strip=False,
    upx=True,
    upx_exclude=[],
    name='musipal',
)
