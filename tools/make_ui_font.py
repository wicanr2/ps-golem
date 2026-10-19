#!/usr/bin/env python3
"""在既有 Pillow 11.3.0 Docker 映像內生成 UI 字圖集，契約見 020。"""
import argparse, hashlib, json, math, struct
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont
import PIL

p=argparse.ArgumentParser()
p.add_argument('--font',required=True)
p.add_argument('--copyright',required=True)
p.add_argument('--output',required=True)
a=p.parse_args()
src=Path(a.font).read_bytes()
expected='b76b0433203017ca80401b2ee0dd69350349871c4b19d504c34dbdd80541690a'
if hashlib.sha256(src).hexdigest()!=expected: raise SystemExit('字型 SHA-256 不符，請先審查來源版本')
if PIL.__version__!='11.3.0': raise SystemExit('需要 Pillow 11.3.0')
font=ImageFont.truetype(a.font,16,index=3)
if font.getname()!=('Noto Sans CJK TC','Regular'): raise SystemExit('繁體中文字面不符')
sfnt=struct.unpack_from('>I',src,12+3*4)[0]
tables={}
for i in range(struct.unpack_from('>H',src,sfnt+4)[0]):
    tag,checksum,offset,length=struct.unpack_from('>4sIII',src,sfnt+12+i*16)
    tables[tag]=(offset,length)
offset,length=tables[b'name']
fmt,count,string_offset=struct.unpack_from('>HHH',src,offset)
names={}
for i in range(count):
    platform,encoding,language,nid,size,start=struct.unpack_from('>6H',src,offset+6+i*12)
    if platform not in (0,3) or nid not in (0,5): continue
    value=src[offset+string_offset+start:offset+string_offset+start+size].decode('utf-16-be')
    names.setdefault(nid,value)
chars=set(range(32,0x300))
for lo,hi in [(0x2000,0x2070),(0x2190,0x2200),(0x25A0,0x2700),(0x3000,0x3100),(0x4E00,0xA000),(0xFF00,0xFFF0)]: chars.update(range(lo,hi))
chars=sorted(chars)
cols,cw,ch=128,20,24
body_height=math.ceil(len(chars)/cols)*ch
atlas=Image.new('L',(cols*cw,body_height+32))
d=ImageDraw.Draw(atlas)
glyphs={}
for i,code in enumerate(chars):
    char=chr(code);x=(i%cols)*cw;y=(i//cols)*ch
    d.text((x,y+20),char,font=font,fill=255,anchor='ls')
    glyphs[char]={'x':x,'y':y,'w':cw,'h':ch,'advance':max(1,math.ceil(font.getlength(char)))}
title_font=ImageFont.truetype(a.font,23,index=3)
title_glyphs={}
for i,char in enumerate('手把設定?'):
    x=i*32;y=body_height
    d.text((x,y+28),char,font=title_font,fill=255,anchor='ls')
    title_glyphs[char]={'x':x,'y':y,'w':32,'h':32,'advance':math.ceil(title_font.getlength(char))}
out=Path(a.output)
if not out.is_dir(): raise SystemExit('輸出目錄須已存在')
atlas.save(out/'ui-font.png',optimize=False)
meta={'height':ch,'title_height':32,'title_glyphs':title_glyphs,'source_sha256':expected,'source_face':3,'source_version':names.get(5,''),'pillow':PIL.__version__,'glyphs':glyphs}
(out/'ui-font.json').write_text(json.dumps(meta,ensure_ascii=False,separators=(',',':'))+'\n')
copyright=Path(a.copyright).read_text()
marker='License: SIL-1.1\n'
license_text=copyright.split(marker)[-1].split('\nLicense: GPL-3+')[0]
license_text='\n'.join('' if line.strip()=='.' else line.removeprefix(' ') for line in license_text.splitlines()).strip()
(out/'ui-font-OFL.txt').write_text('ps-golem UI bitmap font\nDerived from Noto Sans CJK TC Regular\n'+names.get(0,'')+'\n\n'+license_text+'\n')
print(json.dumps({'glyphs':len(glyphs),'size':atlas.size,'version':names.get(5,''),'copyright':names.get(0,'')},ensure_ascii=False))
