"""005 §9：在 Docker 內組裝三平台乾淨封包，不讀原版素材。"""
from pathlib import Path
import hashlib,plistlib,re,shutil,sys,tarfile,zipfile

ver=sys.argv[1]
if not re.fullmatch(r'v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}',ver):raise SystemExit('完整版號格式 v.X.Y.Z-YYYYMMDD')
core=ver[2:].rsplit('-',1)[0]
stage=Path('workplace')/f'stage-{ver}'
out=Path('dist-all')/ver/'patch'
assert stage.is_dir() and out.is_dir()
assert stage.stat().st_uid==out.stat().st_uid==1000
app=stage/'macos/ps-golem.app/Contents'
(app/'MacOS').mkdir(parents=True)
(app/'Resources').mkdir()
shutil.move(stage/'macos/bin/psgolem',app/'MacOS/psgolem')
shutil.move(stage/'macos/bin/psexe',stage/'macos/psexe')
(stage/'macos/bin').rmdir()
(app/'Info.plist').write_bytes(plistlib.dumps({'CFBundleExecutable':'psgolem','CFBundleIdentifier':'io.github.wicanr2.psgolem','CFBundleName':'ps-golem','CFBundlePackageType':'APPL','CFBundleShortVersionString':core,'CFBundleVersion':core,'PSGolemReleaseVersion':ver,'LSMinimumSystemVersion':'11.0','NSHighResolutionCapable':True}))
shutil.copy2('frontend/ebiten/ui-font-OFL.txt',app/'Resources/UI-FONT-LICENSE.txt')
for platform in ('linux','windows','macos'):
    root=stage/platform
    for name in ('LICENSE','README.md'):shutil.copy2(name,root/name)
    shutil.copy2('frontend/ebiten/ui-font-OFL.txt',root/'UI-FONT-LICENSE.txt')
    (root/'docs').mkdir()
    shutil.copy2('docs/play.md',root/'docs/play.md')
for p in stage.rglob('*'):
    if p.is_symlink():raise RuntimeError(f'不接受符號連結 {p}')
    if p.is_file() and (p.suffix.lower() in {'.bin','.cue','.iso','.img','.mcd','.mcr','.chd','.state'} or p.stat().st_size==512*1024):raise RuntimeError(f'疑似原版素材 {p}')
bins=[stage/'linux/psgolem',stage/'linux/psexe',stage/'windows/psgolem.exe',stage/'windows/psexe.exe',stage/'macos/psexe',app/'MacOS/psgolem']
for p in bins:
    b=p.read_bytes()
    if ver.encode() not in b:raise RuntimeError(f'缺少完整版號 {p}')
    if p.name.startswith('psgolem') and b'Drag and drop onto this window' not in b:raise RuntimeError(f'缺少設定畫面 {p}')
archive=out/f'ps-golem-{ver}-linux-amd64.tar.gz'
with tarfile.open(archive,'x:gz') as t:
    def ownership(info):info.uid=info.gid=0;info.uname=info.gname='';return info
    t.add(stage/'linux',arcname='.',filter=ownership)
for platform,arch in (('windows','amd64'),('macos','universal')):
    with zipfile.ZipFile(out/f'ps-golem-{ver}-{platform}-{arch}.zip','x',zipfile.ZIP_DEFLATED) as z:
        for p in sorted((stage/platform).rglob('*')):z.write(p,p.relative_to(stage/platform))
files=sorted(out.iterdir())
(out/'SHA256SUMS').write_text(''.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+p.name+'\n' for p in files if p.is_file()))
print('三平台乾淨封包：版號、設定畫面、字型授權與素材排除通過')
