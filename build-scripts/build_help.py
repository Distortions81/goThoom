#!/usr/bin/env python3
"""Build static figures and a control reference from the client's own render output.
Only marked figure/metadata blocks in the hand-written manual are regenerated.
"""
import argparse
from datetime import date
from html import escape, unescape
from html.parser import HTMLParser
import json
from pathlib import Path
import re
import shutil

ROOT = Path(__file__).resolve().parents[1]
HELP = ROOT / 'website/help'
MANUAL_PAGES = ('index.html', 'automation.html', 'performance.html')
# Editorial explanations are separate from generated UI metadata. No explanation,
# no automatic box. Coordinates still come from the actual rendered control.
ANNOTATIONS = json.loads((HELP / 'annotations.json').read_text())

def selected_controls(screen):
    out = []
    for annotation in ANNOTATIONS.get(screen['id'], []):
        label = annotation['control']
        explanation = annotation['explanation'].strip()
        if not explanation or explanation == label:
            raise ValueError(f"A callout needs an explanation beyond its label: {screen['id']}: {label}")
        matches = [c for c in screen['controls'] if c['label'] == label]
        if not matches:
            raise ValueError(f"Highlight target disappeared: {screen['id']}: {label}")
        out.append(dict(matches[0], explanation=explanation))
    return out

def figure(screen):
    sid, w, h = screen['id'], screen['width'], screen['height']
    controls = selected_controls(screen)
    boxes, legends = [], []
    for n, control in enumerate(controls, 1):
        x,y,cw,ch = control['rect']
        style = f'left:{100*x/w:.4f}%;top:{100*y/h:.4f}%;width:{100*cw/w:.4f}%;height:{100*ch/h:.4f}%'
        boxes.append(f'<span class="help-highlight" style="{style}" aria-hidden="true"><b>{n}</b></span>')
        legends.append(f'<li><b>{n}</b><p>{escape(control["explanation"])}</p></li>')
    overlay = f'<div class="help-highlights">{"".join(boxes)}</div>' if boxes else ''
    legend = f'<ol class="help-legend">{"".join(legends)}</ol>' if legends else ''
    toggle = '<button type="button" class="help-toggle" aria-pressed="true">Hide highlights</button>' if boxes else ''
    alt = screen['title'] + '.'
    if controls:
        alt += ' Highlighted controls: ' + '; '.join(c['label'] for c in controls) + '.'
    return f'''<figure class="help-figure" data-screen="{sid}">
  <div class="help-image" style="max-width:{w}px">
    <img src="images/{sid}.png" width="{w}" height="{h}" loading="lazy" decoding="async" alt="{escape(alt)}">
{overlay}
  </div>
  <figcaption><strong>{escape(screen['route'])}</strong>
{legend}
    <div class="help-image-actions">{toggle}<a href="images/{sid}.png" target="_blank" rel="noopener">Open full-size image</a></div>
  </figcaption>
</figure>'''

def reference_section(screen):
    # Closed menus and slider endpoints are useful reference data that the
    # screenshot cannot show. Do not pad out a table with obvious button advice.
    rows=[]
    for c in screen['controls']:
        if c.get('options') and len(c['options']) > 1:
            details = escape(' · '.join(c['options']))
        elif c['kind'] == 'Slider' and '(range ' in c.get('value',''):
            details = escape(c['value'].split('(range ',1)[1].rstrip(')'))
        else:
            continue
        rows.append(f'<tr><th scope="row">{escape(c["label"])}</th><td>{details}</td></tr>')
    choices = ''
    if rows:
        choices = f'''<details class="help-controls"><summary>Menu options and slider ranges</summary><div class="manual-table-wrap"><table><thead><tr><th>Control</th><th>Available choices or range</th></tr></thead><tbody>{''.join(rows)}</tbody></table></div></details>'''
    return f'''<section class="manual-section" id="{screen['id']}"><h2>{escape(screen['title'])}</h2>
{figure(screen)}
{choices}</section>'''

class SearchParser(HTMLParser):
    def __init__(self, page):
        super().__init__(); self.page=page;self.items=[];self.current=None;self.heading=False;self.in_article=False;self.depth=0
    def handle_starttag(self,tag,attrs):
        a=dict(attrs)
        if tag=='article':self.in_article=True
        if not self.in_article:return
        if tag=='section' and a.get('id'):self.section=a['id']
        if tag in ('h2','h3'):
            self.current={'title':'','url':self.page+'#'+a.get('id',getattr(self,'section','top')),'text':''};self.items.append(self.current);self.heading=True
    def handle_endtag(self,tag):
        if tag in ('h2','h3'):self.heading=False
        if tag=='article':self.in_article=False
    def handle_data(self,text):
        if self.in_article and self.current:
            if self.heading:self.current['title']+=text
            else:self.current['text']+=' '+text

def search_index(manual,screens):
    entries=[]
    for filename in MANUAL_PAGES:
        parser=SearchParser('./' if filename == 'index.html' else filename)
        parser.feed(manual if filename == 'index.html' else (HELP/filename).read_text())
        entries.extend(parser.items)
    for s in screens:
        entries.append({'title':s['title'],'url':'reference.html#'+s['id'],'text':s['route']+' '+' '.join(c['label']+' '+c.get('help','') for c in s['controls'])+' '+' '.join(c['explanation'] for c in selected_controls(s))})
    for entry in entries:entry['text']=' '.join(entry['text'].split())
    return entries

def area_navigation(current):
    areas=(('index.html','Player manual'),('automation.html','Macros, scripts & hotkeys'),('performance.html','Performance & visuals'),('reference.html','UI reference'))
    links=[]
    for filename,label in areas:
        active=' aria-current="page"' if filename == current else ''
        links.append(f'<a href="{filename}"{active}>{escape(label)}</a>')
    return '<nav class="help-area-nav" aria-label="Manual areas">'+''.join(links)+'</nav>'

def render_manual(manual,note,screens,filename):
    used_ids=set(re.findall(r'\bid="([^"]+)"',manual))
    def heading_id(match):
        title=unescape(re.sub('<[^>]+>','',match[2]))
        stem=re.sub('[^a-z0-9]+','-',title.lower()).strip('-') or 'topic'
        candidate=stem; suffix=2
        while candidate in used_ids:
            candidate=f'{stem}-{suffix}';suffix+=1
        used_ids.add(candidate)
        return f'<h{match[1]} id="{candidate}">{match[2]}</h{match[1]}>'
    manual=re.sub(r'<h([23])>(.*?)</h\1>',heading_id,manual,flags=re.S)
    manual=re.sub(r'<!-- help-metadata -->.*?<!-- /help-metadata -->',f'<!-- help-metadata --><p class="manual-version">{note}</p><!-- /help-metadata -->',manual,flags=re.S)
    manual=re.sub(r'<!-- help-navigation -->.*?<!-- /help-navigation -->','<!-- help-navigation -->'+area_navigation(filename)+'<!-- /help-navigation -->',manual,flags=re.S)
    def replace_figure(match):
        sid=match[1]
        return f'<!-- help-figure:{sid} -->\n{figure(screens[sid])}\n<!-- /help-figure -->'
    return re.sub(r'<!-- help-figure:([\w-]+) -->.*?<!-- /help-figure -->',replace_figure,manual,flags=re.S)

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('capture',type=Path)
    parser.add_argument('--updated',type=date.fromisoformat,default=date.today())
    args=parser.parse_args()
    data=json.loads((args.capture/'capture.json').read_text())
    data['updated']=args.updated.isoformat()
    screens={s['id']:s for s in data['screens']}
    unknown = set(ANNOTATIONS) - set(screens)
    if unknown:
        raise ValueError(f"Callouts reference unknown screenshots: {sorted(unknown)}")
    for screen in screens.values():selected_controls(screen)
    note=f'Updated for goThoom test {data["version"]} · Clan Lord {data["clVersion"]} · {args.updated.strftime("%B")} {args.updated.day}, {args.updated.year}'
    for filename in MANUAL_PAGES:
        path=HELP/filename
        path.write_text(render_manual(path.read_text(),note,screens,filename))
    manual=(HELP/'index.html').read_text()
    header=manual[:manual.index('    <main')]
    header=header.replace('<title>User manual — goThoom</title>','<title>Visual control reference — goThoom</title>').replace('https://gothoom.m45sci.xyz/help"','https://gothoom.m45sci.xyz/help/reference.html"')
    links=''.join(f'<a href="#{s["id"]}">{escape(s["title"])}</a>' for s in screens.values())
    reference=header+f'''    <main id="top" class="manual-main help-reference">
      {area_navigation('reference.html')}
      <a class="back-link" href="./">← Back to the step-by-step manual</a>
      <h1>Visual control reference</h1><p class="manual-version">{note}</p>
      <p>Real client windows, with explanations of behavior and tradeoffs that are easy to miss. These examples use the default theme and illustrative data. Installed voices, resources, and your settings may change what is available.</p>
      <p>Numbered callouts explain when an option helps, what it affects, and why it may behave unexpectedly. Straightforward controls are left unboxed. Expand menu options and slider ranges when you need values that are not visible in the screenshot.</p>
      <details class="help-reference-nav" open><summary>Jump to a window</summary><nav aria-label="Reference windows">{links}</nav></details>
      <article class="manual-content">{''.join(reference_section(s) for s in screens.values())}</article>
      <p><a href="#top">Back to top</a> · <a href="./">Return to the manual</a></p>
    </main><script src="manual.js" defer></script></body></html>'''
    (HELP/'reference.html').write_text(reference)
    for sid in screens:
        source=args.capture/(sid+'.png');dest=HELP/'images'/(sid+'.png')
        if source.resolve()!=dest.resolve():shutil.copyfile(source,dest)
    (HELP/'images/capture.json').write_text(json.dumps(data,indent=2)+'\n')
    (HELP/'search-index.json').write_text(json.dumps(search_index(manual,data['screens']),ensure_ascii=False,indent=2)+'\n')
    print(f'Built {len(screens)} screenshots, visual reference, and search index. {note}')

if __name__=='__main__':main()
