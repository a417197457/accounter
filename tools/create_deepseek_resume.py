from docx import Document
from docx.shared import Inches, Pt, RGBColor
from docx.enum.section import WD_SECTION
from docx.enum.text import WD_ALIGN_PARAGRAPH, WD_BREAK, WD_LINE_SPACING
from docx.enum.table import WD_CELL_VERTICAL_ALIGNMENT
from docx.oxml import OxmlElement
from docx.oxml.ns import qn


OUTPUT = "/Users/zhuoyu.yzy/GolangProject/accounter/杨卓宇_DeepSeek服务端开发工程师简历_v4.docx"
BLACK = "000000"
GRAY = "555555"
LIGHT = "D9D9D9"
ACCENT = "17365D"


def set_cell_margins(cell, top=80, start=90, bottom=80, end=90):
    tc = cell._tc
    tcPr = tc.get_or_add_tcPr()
    tcMar = tcPr.first_child_found_in("w:tcMar")
    if tcMar is None:
        tcMar = OxmlElement("w:tcMar")
        tcPr.append(tcMar)
    for m, v in (("top", top), ("start", start), ("bottom", bottom), ("end", end)):
        node = tcMar.find(qn(f"w:{m}"))
        if node is None:
            node = OxmlElement(f"w:{m}")
            tcMar.append(node)
        node.set(qn("w:w"), str(v))
        node.set(qn("w:type"), "dxa")


def set_cell_shading(cell, fill):
    tcPr = cell._tc.get_or_add_tcPr()
    shd = tcPr.find(qn("w:shd"))
    if shd is None:
        shd = OxmlElement("w:shd")
        tcPr.append(shd)
    shd.set(qn("w:fill"), fill)


def set_cell_borders(cell, color=LIGHT, size="4"):
    tcPr = cell._tc.get_or_add_tcPr()
    borders = tcPr.first_child_found_in("w:tcBorders")
    if borders is None:
        borders = OxmlElement("w:tcBorders")
        tcPr.append(borders)
    for edge in ("top", "left", "bottom", "right", "insideH", "insideV"):
        tag = borders.find(qn(f"w:{edge}"))
        if tag is None:
            tag = OxmlElement(f"w:{edge}")
            borders.append(tag)
        tag.set(qn("w:val"), "single")
        tag.set(qn("w:sz"), size)
        tag.set(qn("w:color"), color)


def set_repeat_table_header(row):
    trPr = row._tr.get_or_add_trPr()
    tblHeader = OxmlElement("w:tblHeader")
    tblHeader.set(qn("w:val"), "true")
    trPr.append(tblHeader)


def set_fixed_table_widths(table, widths_inches):
    """Make Word and LibreOffice honor non-equal column widths."""
    table.autofit = False
    tblPr = table._tbl.tblPr
    layout = tblPr.find(qn("w:tblLayout"))
    if layout is None:
        layout = OxmlElement("w:tblLayout")
        tblPr.append(layout)
    layout.set(qn("w:type"), "fixed")
    total_twips = int(sum(widths_inches) * 1440)
    tblW = tblPr.find(qn("w:tblW"))
    if tblW is None:
        tblW = OxmlElement("w:tblW")
        tblPr.append(tblW)
    tblW.set(qn("w:w"), str(total_twips))
    tblW.set(qn("w:type"), "dxa")
    grid_cols = table._tbl.tblGrid.gridCol_lst
    for grid_col, width in zip(grid_cols, widths_inches):
        grid_col.set(qn("w:w"), str(int(width * 1440)))
    for row in table.rows:
        for cell, width in zip(row.cells, widths_inches):
            cell.width = Inches(width)
            tcW = cell._tc.get_or_add_tcPr().find(qn("w:tcW"))
            if tcW is None:
                tcW = OxmlElement("w:tcW")
                cell._tc.get_or_add_tcPr().append(tcW)
            tcW.set(qn("w:w"), str(int(width * 1440)))
            tcW.set(qn("w:type"), "dxa")


def set_run_font(run, size=10.5, bold=False, color=BLACK, east_asia="Arial Unicode MS"):
    run.font.name = "Arial"
    run.font.size = Pt(size)
    run.font.bold = bold
    run.font.color.rgb = RGBColor.from_string(color)
    rPr = run._element.get_or_add_rPr()
    rFonts = rPr.rFonts
    if rFonts is None:
        rFonts = OxmlElement("w:rFonts")
        rPr.insert(0, rFonts)
    rFonts.set(qn("w:ascii"), "Arial")
    rFonts.set(qn("w:hAnsi"), "Arial")
    rFonts.set(qn("w:eastAsia"), east_asia)


def format_paragraph(p, before=0, after=0, line=1.15, keep=False):
    pf = p.paragraph_format
    pf.space_before = Pt(before)
    pf.space_after = Pt(after)
    pf.line_spacing = line
    pf.keep_together = keep


def add_section_heading(doc, text):
    p = doc.add_paragraph(style="Heading 1")
    p.paragraph_format.space_before = Pt(8)
    p.paragraph_format.space_after = Pt(4)
    p.paragraph_format.keep_with_next = True
    r = p.add_run(text)
    set_run_font(r, 12, True, BLACK, "Arial Unicode MS")
    return p


def add_bullet(doc, text, indent=0.2, after=2.2):
    p = doc.add_paragraph(style="List Bullet")
    p.paragraph_format.left_indent = Inches(indent)
    p.paragraph_format.first_line_indent = Inches(-0.16)
    p.paragraph_format.space_after = Pt(after)
    p.paragraph_format.line_spacing = 1.12
    r = p.add_run(text)
    set_run_font(r, 9.6, False, BLACK, "Arial Unicode MS")
    return p


def add_role(doc, title, dates):
    t = doc.add_table(rows=1, cols=2)
    set_fixed_table_widths(t, [5.15, 1.75])
    t.cell(0, 0).vertical_alignment = WD_CELL_VERTICAL_ALIGNMENT.CENTER
    t.cell(0, 1).vertical_alignment = WD_CELL_VERTICAL_ALIGNMENT.CENTER
    for c in t.rows[0].cells:
        set_cell_margins(c, 30, 0, 20, 0)
        tcPr = c._tc.get_or_add_tcPr()
        borders = OxmlElement("w:tcBorders")
        for edge in ("top", "left", "bottom", "right", "insideH", "insideV"):
            tag = OxmlElement(f"w:{edge}")
            tag.set(qn("w:val"), "nil")
            borders.append(tag)
        tcPr.append(borders)
    p1 = t.cell(0, 0).paragraphs[0]
    p2 = t.cell(0, 1).paragraphs[0]
    p2.alignment = WD_ALIGN_PARAGRAPH.RIGHT
    set_run_font(p1.add_run(title), 10.2, True, BLACK, "Arial Unicode MS")
    set_run_font(p2.add_run(dates), 9.3, False, GRAY, "Arial Unicode MS")
    return t


doc = Document()
section = doc.sections[0]
section.page_width = Inches(8.5)
section.page_height = Inches(11)
section.top_margin = Inches(0.48)
section.bottom_margin = Inches(0.48)
section.left_margin = Inches(0.65)
section.right_margin = Inches(0.65)

styles = doc.styles
styles["Normal"].font.name = "Arial"
styles["Normal"]._element.rPr.rFonts.set(qn("w:eastAsia"), "Arial Unicode MS")
styles["Normal"].font.size = Pt(9.6)
styles["Title"].font.color.rgb = RGBColor(0, 0, 0)
styles["Heading 1"].font.color.rgb = RGBColor(0, 0, 0)
title_style_ppr = styles["Title"]._element.get_or_add_pPr()
title_style_border = title_style_ppr.find(qn("w:pBdr"))
if title_style_border is not None:
    title_style_ppr.remove(title_style_border)

p = doc.add_paragraph(style="Title")
p.alignment = WD_ALIGN_PARAGRAPH.CENTER
p.paragraph_format.space_after = Pt(2)
# Remove any border inherited from the built-in Title style.
title_ppr = p._p.get_or_add_pPr()
title_border = title_ppr.find(qn("w:pBdr"))
if title_border is not None:
    title_ppr.remove(title_border)
r = p.add_run("杨卓宇")
set_run_font(r, 20, True, BLACK, "Arial Unicode MS")

p = doc.add_paragraph()
p.alignment = WD_ALIGN_PARAGRAPH.CENTER
p.paragraph_format.space_after = Pt(3)
r = p.add_run("服务端开发工程师  大模型研究平台方向")
set_run_font(r, 11.2, True, BLACK, "Arial Unicode MS")

p = doc.add_paragraph()
p.alignment = WD_ALIGN_PARAGRAPH.CENTER
p.paragraph_format.space_after = Pt(6)
for i, text in enumerate(["15600103320", "zhuoyu_yang@163.com", "北京"]):
    if i:
        rr = p.add_run("  |  ")
        set_run_font(rr, 9, False, GRAY, "Arial Unicode MS")
    rr = p.add_run(text)
    set_run_font(rr, 9, False, GRAY, "Arial Unicode MS")

add_section_heading(doc, "个人概述")
p = doc.add_paragraph()
format_paragraph(p, after=3, line=1.15)
summary = (
    "5 年以上云计算与智算基础设施研发经验，现任阿里云 IaaS 研发团队技术专家。长期负责裸机、GPU、DPU 与超节点服务器的自动化装机、配置、纳管、调度和运维平台建设，具备从 0 到 1 设计并落地核心系统的经验。"
    "主导或深度参与裸机管理平台、智算服务器管理平台、AIStack 一体机与万卡集群云助手建设，并将 Agent、中心网关和 Skill 应用于租户及运维场景；自研并上线 AI 记账微信小程序账小鸭。熟练使用 Go，关注分布式系统、平台抽象、稳定性、可观测性与工程自动化，希望将大规模基础设施平台经验应用于 LLM 研究与训练平台。"
)
set_run_font(p.add_run(summary), 9.8, False, BLACK, "Arial Unicode MS")

add_section_heading(doc, "核心能力")
table = doc.add_table(rows=4, cols=2)
widths = [1.05, 5.85]
set_fixed_table_widths(table, widths)
skills = [
    ("编程与架构", "Go；服务端系统设计；平台能力抽象；管控面与 Agent 架构；API 与模块化设计"),
    ("分布式与平台", "大规模服务器纳管；自动化装机；集群调度；任务编排；高可用；可扩展性；可观测性"),
    ("智算基础设施", "GPU 服务器；高性能存储与网络；GPU 驱动；DPU；超节点；DHCP；PXE；裸机运维"),
    ("AI 与 LLM", "DeepSeek R1 部署；智算运维 Agent；中心网关；Skill 建设；自然语言运维；AI 应用开发"),
]
for i, (k, v) in enumerate(skills):
    for j, cell in enumerate(table.rows[i].cells):
        cell.width = Inches(widths[j])
        cell.vertical_alignment = WD_CELL_VERTICAL_ALIGNMENT.CENTER
        set_cell_margins(cell, top=85, start=90, bottom=85, end=90)
        set_cell_borders(cell)
        if j == 0:
            set_cell_shading(cell, "EAF0F8")
        elif i % 2 == 1:
            set_cell_shading(cell, "F7F8FA")
    p0 = table.cell(i, 0).paragraphs[0]
    p1 = table.cell(i, 1).paragraphs[0]
    p0.alignment = WD_ALIGN_PARAGRAPH.CENTER
    set_run_font(p0.add_run(k), 9.2, True, ACCENT, "Arial Unicode MS")
    set_run_font(p1.add_run(v), 9.2, False, BLACK, "Arial Unicode MS")
    format_paragraph(p0, line=1.05)
    format_paragraph(p1, line=1.05)

add_section_heading(doc, "工作经历")
add_role(doc, "阿里云  IaaS 研发团队  技术专家", "2025.01 - 至今")
add_bullet(doc, "从 0 到 1 完成 AIStack 一体机研发，负责一体机自动化装配及 AIStack 平台搭建；2025 年 3 月上线，支持单台 16 卡 PPU 部署满血版 DeepSeek R1 模型。")
add_bullet(doc, "扩展智算管理平台的异构硬件能力，完成 DPU、超节点等新型服务器的装机、适配与纳管，使平台能够统一执行自动化管理与调度。")
add_bullet(doc, "从 0 到 1 开发裸机云助手，面向万卡规模集群提供秒级运维能力，缩短大规模机器状态查询与运维操作链路。")
add_bullet(doc, "探索智算平台专用 Agent，完成 Agent 搭建、中心网关开发与 Skill 建设，区分租户侧和运维侧权限及管控边界。")
add_bullet(doc, "将 Agent 与云助手联动，支持问题自动定位、自然语言查询集群及机器状态，并支持管理员通过一句话触发运维操作。")

add_role(doc, "阿里云  IaaS 研发团队  高级开发工程师", "2022.04 - 2025.01")
add_bullet(doc, "负责智算服务器管理平台研发，在裸机管理能力之上建设 GPU 服务器自动化配置流程，覆盖高性能存储、高性能网络与 GPU 驱动的安装配置。")
add_bullet(doc, "建设集群级调度能力，将单机装配与纳管能力扩展至集群场景，支撑多个万卡规模集群客户的交付与运行。")
add_bullet(doc, "围绕智算资源的统一管理持续演进平台能力，推动复杂硬件配置流程标准化、自动化并沉淀为可复用模块。")

doc.add_page_break()
add_role(doc, "阿里云  IaaS 研发团队  开发工程师", "2020.07 - 2022.04")
add_bullet(doc, "从 0 到 1 参与并完成裸机管理平台研发，使用 Go 开发管控面与裸机 Agent，提供裸机自动化装机、配置、监控和运维能力。")
add_bullet(doc, "开发云底座克隆装机服务，建设 DHCP 与 PXE 服务，支持服务器自动分配带外 IP，并通过 PXE 完成网络装机。")
add_bullet(doc, "维护 Ubuntu、Alibaba Cloud Linux 3、CentOS、Kylin 等公共镜像，保障不同操作系统在裸机场景下的稳定交付。")

add_section_heading(doc, "代表性项目")
projects = [
    ("AIStack 一体机与 DeepSeek R1 部署", "面向一体化 AI 交付场景，负责自动化装配和平台搭建，使单台 16 卡 PPU 设备具备满血版 DeepSeek R1 的部署能力，并按期于 2025 年 3 月上线。"),
    ("万卡集群裸机云助手", "将海量机器的查询、诊断与运维动作封装为平台能力，以秒级响应支撑万卡规模集群运维；进一步接入 Agent，实现自然语言查询、问题定位和运维操作。"),
    ("智算服务器统一管理平台", "将裸机生命周期管理扩展到 GPU、DPU 与超节点服务器，统一编排装机、驱动、高性能网络和存储配置，并建设集群调度能力。"),
    ("账小鸭 AI 记账微信小程序", "自研并上线 AI 记账产品，将自然语言理解应用于个人账本场景，支持用户通过一句话完成记账，并通过一句话总结账本，降低结构化录入与账目分析的使用门槛。"),
]
for name, body in projects:
    p = doc.add_paragraph()
    p.paragraph_format.space_after = Pt(2.4)
    p.paragraph_format.line_spacing = 1.12
    set_run_font(p.add_run(name + "  "), 9.6, True, BLACK, "Arial Unicode MS")
    set_run_font(p.add_run(body), 9.4, False, BLACK, "Arial Unicode MS")

add_section_heading(doc, "教育经历")
edu = doc.add_table(rows=2, cols=3)
edu_data = [
    ("北京邮电大学", "计算机技术  硕士", "2017 - 2020"),
    ("东北农业大学", "物联网工程  本科", "2013 - 2017"),
]
edu_widths = [2.3, 2.8, 1.8]
set_fixed_table_widths(edu, edu_widths)
for i, row in enumerate(edu.rows):
    for j, cell in enumerate(row.cells):
        cell.width = Inches(edu_widths[j])
        cell.vertical_alignment = WD_CELL_VERTICAL_ALIGNMENT.CENTER
        set_cell_margins(cell, 70, 70, 70, 70)
        set_cell_borders(cell)
        if i % 2 == 1:
            set_cell_shading(cell, "F5F7FA")
        p = cell.paragraphs[0]
        p.alignment = WD_ALIGN_PARAGRAPH.CENTER
        set_run_font(p.add_run(edu_data[i][j]), 9.3, j == 0, BLACK, "Arial Unicode MS")

# Prevent table rows from splitting across pages.
for tbl in doc.tables:
    for row in tbl.rows:
        trPr = row._tr.get_or_add_trPr()
        cant_split = OxmlElement("w:cantSplit")
        trPr.append(cant_split)

# Document metadata deliberately generic until the candidate name is supplied.
doc.core_properties.title = "DeepSeek 服务端开发工程师简历"
doc.core_properties.subject = "大模型研究平台方向"
doc.core_properties.author = "杨卓宇"
doc.core_properties.keywords = "Go, 分布式系统, 智算平台, 裸机, GPU, Agent, LLM"

doc.save(OUTPUT)
print(OUTPUT)
