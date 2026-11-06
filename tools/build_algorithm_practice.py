#!/usr/bin/env python3
import html
import json
import re
import subprocess
from pathlib import Path
from urllib.parse import urlparse

ROOT = Path('/Users/zhuoyu.yzy/GolangProject/accounter')
REPO = Path('/tmp/leetcode-src.v2eIqC')
NEETCODE = Path('/tmp/neetcode150.json')
LEETCODE = Path('/tmp/leetcode_all.json')
OUT_DIR = ROOT / 'algorithm-practice'
OUT = OUT_DIR / 'index.html'
README_CACHE = Path('/tmp/leet_readmes')
SELECTED_PATHS = Path('/tmp/selected_paths.txt')

QUOTAS = {
    'Arrays & Hashing': 7, 'Two Pointers': 5, 'Sliding Window': 6,
    'Stack': 6, 'Binary Search': 6, 'Linked List': 6, 'Trees': 7,
    'Heap / Priority Queue': 5, 'Backtracking': 5, 'Tries': 3,
    'Graphs': 7, 'Advanced Graphs': 4, '1-D Dynamic Programming': 7,
    '2-D Dynamic Programming': 6, 'Greedy': 5, 'Intervals': 5,
    'Math & Geometry': 5, 'Bit Manipulation': 5,
}

CATEGORY_ZH = {
    'Arrays & Hashing': '数组与哈希', 'Two Pointers': '双指针',
    'Sliding Window': '滑动窗口', 'Stack': '栈', 'Binary Search': '二分查找',
    'Linked List': '链表', 'Trees': '二叉树', 'Heap / Priority Queue': '堆与优先队列',
    'Backtracking': '回溯', 'Tries': '字典树', 'Graphs': '图',
    'Advanced Graphs': '高级图算法', '1-D Dynamic Programming': '一维动态规划',
    '2-D Dynamic Programming': '二维动态规划', 'Greedy': '贪心',
    'Intervals': '区间', 'Math & Geometry': '数学与几何', 'Bit Manipulation': '位运算',
}


def git_show(path: str) -> str:
    if path.endswith('/README.md') and README_CACHE.exists() and SELECTED_PATHS.exists():
        folder = path[:-len('/README.md')]
        folders = SELECTED_PATHS.read_text().splitlines()
        if folder in folders:
            cached = README_CACHE / f'{folders.index(folder):03d}.md'
            if cached.exists():
                return cached.read_text(encoding='utf-8')
    return subprocess.check_output(
        ['git', '-C', str(REPO), 'show', f'HEAD:{path}'], text=True,
        stderr=subprocess.DEVNULL,
    )


def plain_text(raw: str, keep_examples: bool = True) -> str:
    if not keep_examples:
        raw = re.sub(r'<pre[\s\S]*?</pre>', ' ', raw, flags=re.I)
    raw = re.sub(r'<[^>]+>', ' ', raw)
    raw = html.unescape(raw)
    raw = re.sub(r'\$([^$]+)\$', r'\1', raw)
    raw = re.sub(r'[`*_#>-]', '', raw)
    return re.sub(r'\s+', ' ', raw).strip()


def full_problem_text(raw: str) -> str:
    """Convert the complete HTML statement to readable plain text without losing examples."""
    raw = re.sub(r'<br\s*/?>', '\n', raw, flags=re.I)
    raw = re.sub(r'</(?:p|div|pre|li|ul|ol|h\d)>', '\n', raw, flags=re.I)
    raw = re.sub(r'<li[^>]*>', '• ', raw, flags=re.I)
    raw = re.sub(r'<[^>]+>', '', raw)
    raw = html.unescape(raw)
    raw = re.sub(r'\$([^$]+)\$', r'\1', raw)
    lines = [re.sub(r'[ \t]+', ' ', line).strip() for line in raw.splitlines()]
    return '\n'.join(line for line in lines if line).strip()


def extract_code(md: str, language: str) -> str:
    pattern = rf'#### {re.escape(language)}\s*\n```(?:{language.lower()}|golang)\s*\n([\s\S]*?)```'
    m = re.search(pattern, md, flags=re.I)
    return m.group(1).strip() if m else f'// 暂无 {language} 参考实现'


def extract_problem(md: str, fallback_title: str):
    title_match = re.search(r'^# \[(?:\d+\.\s*)?([^\]]+)\]', md, flags=re.M)
    title = title_match.group(1).strip() if title_match else fallback_title
    diff_match = re.search(r'^difficulty:\s*(\S+)', md, flags=re.M)
    difficulty = diff_match.group(1) if diff_match else '中等'
    desc_match = re.search(r'<!-- description:start -->([\s\S]*?)<!-- description:end -->', md)
    full_desc = full_problem_text(desc_match.group(1)) if desc_match else '请根据题目链接阅读完整题意。'
    desc = plain_text(desc_match.group(1), keep_examples=False) if desc_match else full_desc
    desc = desc[:220] + ('…' if len(desc) > 220 else '')
    solution_match = re.search(r'<!-- solution:start -->([\s\S]*?)<!-- tabs:start -->', md)
    idea = plain_text(solution_match.group(1)) if solution_match else '分析数据规模，选择合适的数据结构，并明确状态、边界与复杂度。'
    idea = idea[:520] + ('…' if len(idea) > 520 else '')
    tags_match = re.search(r'tags:\s*\n((?:\s+- .+\n)+)', md)
    tags = re.findall(r'-\s+(.+)', tags_match.group(1)) if tags_match else []
    return title, difficulty, desc, full_desc, idea, tags


def build_problems():
    source = json.loads(NEETCODE.read_text())
    all_data = json.loads(LEETCODE.read_text())
    slug_to_id = {
        row['stat']['question__title_slug']: row['stat']['frontend_question_id']
        for row in all_data['stat_status_pairs']
    }
    tree = subprocess.check_output(
        ['git', '-C', str(REPO), 'ls-tree', '-r', '--name-only', 'HEAD'], text=True
    ).splitlines()
    dir_by_id = {}
    for path in tree:
        m = re.match(r'(solution/\d+-\d+/(\d{4})\.[^/]+)/README\.md$', path)
        if m:
            dir_by_id[int(m.group(2))] = m.group(1)

    selected = []
    for category, quota in QUOTAS.items():
        items = list(source[category].items())[:quota]
        for english_title, meta in items:
            slug = urlparse(meta['url']).path.strip('/').split('/')[-1]
            problem_id = slug_to_id.get(slug)
            if not problem_id or problem_id not in dir_by_id:
                raise RuntimeError(f'Cannot map problem: {english_title} ({slug})')
            folder = dir_by_id[problem_id]
            md = git_show(f'{folder}/README.md')
            title, difficulty, desc, full_desc, idea, tags = extract_problem(md, english_title)
            selected.append({
                'id': problem_id, 'title': title, 'enTitle': english_title,
                'category': CATEGORY_ZH[category], 'difficulty': difficulty,
                'summary': desc, 'description': full_desc, 'idea': idea, 'tags': tags[:4],
                'url': f'https://leetcode.cn/problems/{slug}/',
                'go': extract_code(md, 'Go'), 'java': extract_code(md, 'Java'),
            })
    if len(selected) != 100:
        raise RuntimeError(f'Expected 100 problems, got {len(selected)}')
    return selected


HTML_TEMPLATE = r'''<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width,initial-scale=1" />
  <meta name="description" content="100 道高频算法题，包含分类、思路与 Go、Java 参考答案。" />
  <title>算法百题训练场</title>
  <style>
    :root{--bg:#07111f;--panel:#0d1a2b;--panel2:#101f32;--line:#20344c;--text:#edf5ff;--muted:#91a5bd;--cyan:#35d4c7;--blue:#65a9ff;--amber:#ffc66d;--red:#ff7d8c;--green:#63dfa6;--shadow:0 22px 70px rgba(0,0,0,.28)}
    *{box-sizing:border-box}html{scroll-behavior:smooth}body{margin:0;background:radial-gradient(circle at 8% 0%,#143254 0,transparent 28rem),var(--bg);color:var(--text);font-family:Inter,"PingFang SC","Microsoft YaHei",system-ui,sans-serif;line-height:1.65}
    button,input,select{font:inherit}.shell{max-width:1480px;margin:auto;padding:24px}.topbar{display:flex;align-items:center;justify-content:space-between;gap:20px;margin-bottom:20px}.brand{display:flex;align-items:center;gap:14px}.logo{width:46px;height:46px;display:grid;place-items:center;border:1px solid #3e6682;border-radius:14px;background:linear-gradient(145deg,#183b58,#0a2235);font-weight:900;color:var(--cyan);box-shadow:var(--shadow)}h1{font-size:clamp(1.35rem,2vw,2rem);margin:0;letter-spacing:.02em}.sub{margin:2px 0 0;color:var(--muted);font-size:.9rem}.progressBox{min-width:250px}.progressMeta{display:flex;justify-content:space-between;color:var(--muted);font-size:.85rem}.progress{height:8px;background:#14273b;border-radius:99px;overflow:hidden;margin-top:7px}.progress>i{display:block;height:100%;width:0;background:linear-gradient(90deg,var(--cyan),var(--blue));transition:.3s}
    .workspace{display:grid;grid-template-columns:240px minmax(0,1fr);gap:22px}.sidebar{position:sticky;top:18px;align-self:start;background:rgba(13,26,43,.88);border:1px solid var(--line);border-radius:18px;padding:16px;max-height:calc(100vh - 36px);overflow:auto;box-shadow:var(--shadow);backdrop-filter:blur(14px)}.sideTitle{font-size:.78rem;color:var(--muted);letter-spacing:.14em;text-transform:uppercase;margin:4px 8px 10px}.cat{width:100%;border:0;background:transparent;color:#b9c9d9;text-align:left;padding:9px 10px;border-radius:10px;display:flex;justify-content:space-between;cursor:pointer}.cat:hover,.cat.active{background:#17304a;color:#fff}.cat b{font-size:.78rem;color:#6f8ba7}.cat.active b{color:var(--cyan)}
    .toolbar{position:sticky;top:0;z-index:5;background:linear-gradient(var(--bg) 75%,transparent);padding:0 0 20px;display:grid;grid-template-columns:minmax(240px,1fr) 150px 150px auto;gap:10px}.control{height:44px;border:1px solid var(--line);border-radius:12px;background:#0d1a2b;color:var(--text);padding:0 13px;outline:none}.control:focus{border-color:var(--blue);box-shadow:0 0 0 3px rgba(101,169,255,.12)}.random{border:1px solid #2e6d72;background:#123539;color:#bffff8;font-weight:700;cursor:pointer}.random:hover{background:#17494c}.resultLine{color:var(--muted);font-size:.9rem;margin:0 0 12px}.list{display:grid;gap:14px}.card{border:1px solid var(--line);border-radius:16px;background:linear-gradient(145deg,rgba(16,31,50,.96),rgba(10,23,38,.96));overflow:hidden;box-shadow:0 10px 34px rgba(0,0,0,.13)}.cardHead{display:grid;grid-template-columns:auto 1fr auto;gap:14px;align-items:start;padding:18px}.num{font:800 .8rem/1 ui-monospace,monospace;color:var(--cyan);border:1px solid #2b5b61;padding:7px 8px;border-radius:9px;background:#0b292c}.titleRow{display:flex;gap:10px;align-items:center;flex-wrap:wrap}.titleRow h2{font-size:1.08rem;margin:0}.titleRow a{color:inherit;text-decoration:none}.titleRow a:hover{color:var(--blue)}.meta{display:flex;gap:7px;flex-wrap:wrap;margin-top:8px}.pill{font-size:.76rem;padding:2px 8px;border-radius:999px;background:#182b40;color:#a9bdd1;border:1px solid #273d55}.easy{color:var(--green)}.medium{color:var(--amber)}.hard{color:var(--red)}.summary{color:#bdcadd;margin:12px 0 0;font-size:.93rem}.status{display:flex;gap:7px}.iconBtn{width:38px;height:38px;border:1px solid var(--line);border-radius:10px;background:#102238;color:#8fa7bf;cursor:pointer;font-size:1rem}.iconBtn:hover{border-color:#4d7194;color:#fff}.iconBtn.done{background:#103b31;color:var(--green);border-color:#276c59}.iconBtn.starred{color:#ffd46f;background:#3a3015;border-color:#665622}
    details.statement,details.answer{border-top:1px solid var(--line)}details.statement>summary,details.answer>summary{cursor:pointer;list-style:none;padding:13px 18px;color:#b8cce1;font-weight:700;background:rgba(7,17,31,.32)}details.statement>summary::-webkit-details-marker,details.answer>summary::-webkit-details-marker{display:none}details.statement>summary:after,details.answer>summary:after{content:'＋';float:right;color:var(--cyan)}details.statement[open]>summary:after,details.answer[open]>summary:after{content:'－'}.statementBody{padding:18px;color:#d5e0eb;background:#091625;white-space:pre-wrap;font-size:.93rem;line-height:1.8}.answerBody{padding:18px}.idea{margin:0 0 16px;color:#d5e0eb;background:#0a1726;border-left:3px solid var(--cyan);padding:12px 14px;border-radius:0 10px 10px 0}.tabs{display:flex;gap:8px;margin-bottom:8px}.tab{border:1px solid var(--line);background:#0c1a2a;color:#91a8c1;padding:7px 14px;border-radius:9px;cursor:pointer}.tab.active{color:#07111f;background:var(--cyan);border-color:var(--cyan);font-weight:800}.codeWrap{position:relative}.copy{position:absolute;right:9px;top:9px;border:1px solid #35506c;background:#11253a;color:#aac0d7;border-radius:8px;padding:5px 9px;cursor:pointer}pre{margin:0;max-height:520px;overflow:auto;padding:18px;background:#06101c;border:1px solid #1c3045;border-radius:12px;color:#d8e8f8;font:14px/1.6 "SFMono-Regular",Consolas,monospace;tab-size:4}.empty{border:1px dashed #35506c;border-radius:16px;padding:48px;text-align:center;color:var(--muted)}footer{margin:32px 0 10px;color:#71859b;font-size:.78rem;text-align:center}footer a{color:#91aac4}
    @media(max-width:900px){.workspace{grid-template-columns:1fr}.sidebar{position:static;max-height:none;display:flex;gap:6px;overflow:auto}.sideTitle{display:none}.cat{white-space:nowrap;width:auto}.toolbar{grid-template-columns:1fr 1fr;top:0}.toolbar input{grid-column:1/-1}.topbar{align-items:flex-start;flex-direction:column}.progressBox{width:100%}}
    @media(max-width:560px){.shell{padding:14px}.toolbar{grid-template-columns:1fr}.toolbar input{grid-column:auto}.cardHead{grid-template-columns:auto 1fr}.status{grid-column:2}.summary{font-size:.9rem}.answerBody{padding:12px}}
    @media(prefers-reduced-motion:reduce){html{scroll-behavior:auto}*{transition:none!important}}
  </style>
</head>
<body>
  <div class="shell">
    <header class="topbar">
      <div class="brand"><div class="logo">100</div><div><h1>算法百题训练场</h1><p class="sub">按模式练习，比随机刷题更容易形成解题直觉</p></div></div>
      <div class="progressBox"><div class="progressMeta"><span>完成进度</span><strong id="progressText">0 / 100</strong></div><div class="progress"><i id="progressBar"></i></div></div>
    </header>
    <div class="workspace">
      <aside class="sidebar" id="categories"><div class="sideTitle">知识点</div></aside>
      <main>
        <div class="toolbar">
          <input id="search" class="control" placeholder="搜索题目、标签或编号…" aria-label="搜索题目" />
          <select id="difficulty" class="control" aria-label="难度"><option value="">全部难度</option><option>简单</option><option>中等</option><option>困难</option></select>
          <select id="statusFilter" class="control" aria-label="完成状态"><option value="">全部状态</option><option value="todo">未完成</option><option value="done">已完成</option><option value="star">已收藏</option></select>
          <button id="random" class="control random">随机一题</button>
        </div>
        <p class="resultLine" id="resultLine"></p>
        <section class="list" id="list"></section>
        <footer>选题参考 <a href="https://leetcode.com/studyplan/top-interview-150/" target="_blank" rel="noreferrer">LeetCode Top Interview 150</a> 与 NeetCode 150；思路及代码参考并整理自 <a href="https://github.com/doocs/leetcode" target="_blank" rel="noreferrer">doocs/leetcode</a>，遵循 CC BY-SA 4.0。</footer>
      </main>
    </div>
  </div>
  <script>
    const problems = __PROBLEMS__;
    const saved = JSON.parse(localStorage.getItem('algo100-state') || '{}');
    let category = '全部';
    const esc = s => String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    const state = id => saved[id] || {done:false,star:false};
    function persist(){localStorage.setItem('algo100-state', JSON.stringify(saved)); updateProgress()}
    function updateProgress(){const n=problems.filter(p=>state(p.id).done).length;document.querySelector('#progressText').textContent=`${n} / 100`;document.querySelector('#progressBar').style.width=`${n}%`}
    function renderCategories(){const counts={};problems.forEach(p=>counts[p.category]=(counts[p.category]||0)+1);const root=document.querySelector('#categories');root.innerHTML='<div class="sideTitle">知识点</div>'+['全部',...Object.keys(counts)].map(c=>`<button class="cat ${c===category?'active':''}" data-cat="${c}"><span>${c}</span><b>${c==='全部'?100:counts[c]}</b></button>`).join('');root.querySelectorAll('.cat').forEach(b=>b.onclick=()=>{category=b.dataset.cat;renderCategories();render()})}
    function filtered(){const q=document.querySelector('#search').value.trim().toLowerCase(),d=document.querySelector('#difficulty').value,s=document.querySelector('#statusFilter').value;return problems.filter(p=>(category==='全部'||p.category===category)&&(!d||p.difficulty===d)&&(!q||`${p.id} ${p.title} ${p.enTitle} ${p.tags.join(' ')}`.toLowerCase().includes(q))&&(!s||(s==='done'&&state(p.id).done)||(s==='todo'&&!state(p.id).done)||(s==='star'&&state(p.id).star)))}
    function card(p,i){const st=state(p.id),dc=p.difficulty==='简单'?'easy':p.difficulty==='困难'?'hard':'medium';return `<article class="card" id="p-${p.id}"><div class="cardHead"><div class="num">${String(i+1).padStart(3,'0')}</div><div><div class="titleRow"><h2><a href="${p.url}" target="_blank" rel="noreferrer">${p.id}. ${esc(p.title)}</a></h2></div><div class="meta"><span class="pill">${p.category}</span><span class="pill ${dc}">${p.difficulty}</span>${p.tags.map(t=>`<span class="pill">${esc(t)}</span>`).join('')}</div><p class="summary">${esc(p.summary)}</p></div><div class="status"><button class="iconBtn ${st.star?'starred':''}" data-star="${p.id}" title="收藏" aria-label="收藏">★</button><button class="iconBtn ${st.done?'done':''}" data-done="${p.id}" title="标记完成" aria-label="标记完成">✓</button></div></div><details class="statement"><summary>查看完整题目</summary><div class="statementBody">${esc(p.description)}</div></details><details class="answer"><summary>查看思路与答案</summary><div class="answerBody"><p class="idea"><strong>解题思路：</strong>${esc(p.idea)}</p><div class="tabs"><button class="tab active" data-lang="go">Go</button><button class="tab" data-lang="java">Java</button></div><div class="codeWrap"><button class="copy">复制</button><pre><code data-id="${p.id}">${esc(p.go)}</code></pre></div></div></details></article>`}
    async function copyText(text){if(navigator.clipboard&&window.isSecureContext)return navigator.clipboard.writeText(text);const a=document.createElement('textarea');a.value=text;a.style.position='fixed';a.style.opacity='0';document.body.appendChild(a);a.select();document.execCommand('copy');a.remove()}
    function wire(){document.querySelectorAll('[data-done]').forEach(b=>b.onclick=()=>{const id=b.dataset.done;saved[id]={...state(id),done:!state(id).done};persist();render()});document.querySelectorAll('[data-star]').forEach(b=>b.onclick=()=>{const id=b.dataset.star;saved[id]={...state(id),star:!state(id).star};persist();render()});document.querySelectorAll('.tabs').forEach(t=>t.querySelectorAll('.tab').forEach(b=>b.onclick=()=>{t.querySelectorAll('.tab').forEach(x=>x.classList.toggle('active',x===b));const code=t.nextElementSibling.querySelector('code'),p=problems.find(x=>String(x.id)===code.dataset.id);code.textContent=p[b.dataset.lang]}));document.querySelectorAll('.copy').forEach(b=>b.onclick=async()=>{await copyText(b.nextElementSibling.textContent);b.textContent='已复制';setTimeout(()=>b.textContent='复制',1200)})}
    function render(){const rows=filtered();document.querySelector('#resultLine').textContent=`当前显示 ${rows.length} 道题`;document.querySelector('#list').innerHTML=rows.length?rows.map(card).join(''):'<div class="empty">没有符合条件的题目，换个筛选条件试试。</div>';wire()}
    ['search','difficulty','statusFilter'].forEach(id=>document.querySelector('#'+id).addEventListener(id==='search'?'input':'change',render));document.querySelector('#random').onclick=()=>{const rows=filtered();if(!rows.length)return;const p=rows[Math.floor(Math.random()*rows.length)];document.querySelector('#p-'+p.id)?.scrollIntoView({behavior:'smooth',block:'center'});setTimeout(()=>document.querySelector('#p-'+p.id+' details').open=true,350)};
    renderCategories();render();updateProgress();
  </script>
</body>
</html>'''


def main():
    problems = build_problems()
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    payload = json.dumps(problems, ensure_ascii=False).replace('</', '<\\/')
    OUT.write_text(HTML_TEMPLATE.replace('__PROBLEMS__', payload), encoding='utf-8')
    print(f'Wrote {OUT} with {len(problems)} problems')


if __name__ == '__main__':
    main()
