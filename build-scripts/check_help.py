#!/usr/bin/env python3
"""Check help links, image sizes, annotations, provenance, and generated content."""
from html.parser import HTMLParser
import json
from pathlib import Path
import re
import struct
import sys
sys.dont_write_bytecode = True
from urllib.parse import unquote, urlsplit

ROOT=Path(__file__).resolve().parents[1]
SITE=ROOT/'website'
HELP=SITE/'help'

class Page(HTMLParser):
    def __init__(self,path):
        super().__init__(convert_charrefs=True)
        self.path=path;self.ids=set();self.refs=[];self.images=[];self.errors=[]
        self.feed(path.read_text())
    def handle_starttag(self,tag,attrs):
        attrs=dict(attrs)
        if 'id' in attrs:
            if attrs['id'] in self.ids:self.errors.append('duplicate ID '+attrs['id'])
            self.ids.add(attrs['id'])
        if tag in ('a','link') and attrs.get('href'):self.refs.append(attrs['href'])
        if tag in ('img','script') and attrs.get('src'):self.refs.append(attrs['src'])
        if tag=='img':
            self.images.append(attrs)
            if 'alt' not in attrs:self.errors.append('image has no alt attribute')

def main():
    pages={}
    def page(path):
        if path not in pages:pages[path]=Page(path)
        return pages[path]
    def check_ref(origin,url):
        parsed=urlsplit(url)
        if parsed.scheme or parsed.netloc:return
        if parsed.path.startswith('/'):
            # Existing server-owned endpoints, not generated manual files.
            if parsed.path.startswith('/accounts'):return
            dest=SITE/parsed.path.lstrip('/')
        else:dest=origin if not parsed.path else origin.parent/unquote(parsed.path)
        dest=dest.resolve()
        if dest.is_dir():dest/='index.html'
        if not dest.is_file():raise ValueError(f'{origin.name}: missing link {url}')
        if parsed.fragment and dest.suffix=='.html' and unquote(parsed.fragment) not in page(dest).ids:
            raise ValueError(f'{origin.name}: missing anchor {url}')
    from build_help import MANUAL_PAGES
    for path in (HELP/name for name in (*MANUAL_PAGES, 'reference.html')):
        parsed=page(path.resolve())
        if parsed.errors:raise ValueError(f'{path}: {parsed.errors}')
        for ref in parsed.refs:check_ref(path,ref)
        for img in parsed.images:
            if img['src'].startswith('images/'):
                content=(path.parent/img['src']).read_bytes()
                width,height=struct.unpack('>II',content[16:24])
                if width!=int(img['width']) or height!=int(img['height']):raise ValueError('Stale image dimensions: '+img['src'])
    data=json.loads((HELP/'images/capture.json').read_text())
    manual=(HELP/'index.html').read_text()
    reference=(HELP/'reference.html').read_text()
    manuals={name:(HELP/name).read_text() for name in MANUAL_PAGES}
    for name,content in manuals.items():
        if len(re.findall(r'<!-- help-metadata -->',content))!=1:raise ValueError('Expected one update note in '+name)
    if not re.fullmatch(r'\d{4}-\d{2}-\d{2}',data['updated']):raise ValueError('Missing update date')
    from build_help import figure, selected_controls, reference_section, search_index
    for screen in data['screens']:
        png=HELP/'images'/(screen['id']+'.png')
        if not png.is_file():raise ValueError('Missing capture '+str(png))
        selected_controls(screen)
        for c in screen['controls']:
            x,y,w,h=c['rect']
            if min(x,y)<0 or min(w,h)<=0 or x+w>screen['width'] or y+h>screen['height']:raise ValueError('Invalid highlight bounds '+screen['id'])
        if reference_section(screen) not in reference:raise ValueError('Stale generated reference: '+screen['id'])
        marker='<!-- help-figure:'+screen['id']+' -->'
        for name,content in manuals.items():
            if marker in content and marker+'\n'+figure(screen)+'\n<!-- /help-figure -->' not in content:raise ValueError('Stale figure in '+name+': '+screen['id'])
    for marker in re.findall(r'<!-- help-figure:([\w-]+) -->',''.join(manuals.values())):
        if marker not in {s['id'] for s in data['screens']}:raise ValueError('Unknown capture marker '+marker)
    search=json.loads((HELP/'search-index.json').read_text())
    if search!=search_index(manual,data['screens']):raise ValueError('Stale search index: regenerate help')
    for entry in search:check_ref(HELP/'index.html',entry['url'])
    print(f'Help checks passed: {len(data["screens"])} screenshots, {len(search)} searchable topics; links, image dimensions, and generated callouts match.')

if __name__=='__main__':main()
