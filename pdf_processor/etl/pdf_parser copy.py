
import re
import pymupdf
import json
from collections import defaultdict

def analyze_text_structure(doc):
    font_info = defaultdict(int)

    # 第一遍：收集所有字体大小和字体名
    for page in doc:
        blocks = page.get_text("dict")
        for block in blocks["blocks"]:
            if "lines" in block:
                for line in block["lines"]:
                    for span in line["spans"]:
                        font_size = span["size"]
                        font_name = span["font"]
                        font_info[(font_name, font_size)] += len(span["text"])

    # 识别标题字体大小（通常较大且使用较少）
    sorted_fonts = sorted(font_info.items(), key=lambda x: (-x[0][1], x[1]))

    # 识别正文字体大小和字体名称
    context_font = max(font_info.items(), key=lambda x: x[1])[0]
    title_fonts = [size for size, _ in sorted_fonts[:3]]  # 取前3个最大字体

    # 第二遍：构建结构化内容
    structured_content = []
    current_section = None
    # 遍历每一页
    for page_num, page in enumerate(doc):
        # get blocks
        blocks = page.get_text("dict")

        i = 0
        while i < len(blocks["blocks"]):
            block = blocks["blocks"][i]
            content = ""
            # get block and title if the block contains one
            titles = extract_title_from_block(block, title_fonts)
            stripped_block = extract_content_from_block(block, context_font, title_fonts)

            if "lines" not in block or not check_context_block(block["lines"], context_font, title_fonts)[0]:
                should_trim, re_start = check_consecutive_non_context(blocks, i, context_font, title_fonts)
                if should_trim:
                    i = re_start
                    continue
            # if content exists(title + content or None + content)
            if stripped_block:
                content = stripped_block
                if titles:
                    if current_section:
                        structured_content.append(current_section)
                    for t in titles[:-1]:
                        structured_content.append({
                            "chapter": t,
                            "page": page_num + 1,
                            "content": ""
                        })
                    current_section = {
                        "chapter": titles[-1],
                        "page": page_num + 1,
                        "content": content
                    }
                else:
                    if current_section:
                        current_section["content"] += '\n' + content
                    else:
                        current_section = {
                            "chapter": "",
                            "page": page_num + 1,
                            "content": content
                        }
            # if only found title, then contiune searching
            # This is the case when the title and content aren't in
            # the same block, where title will appear alone
            elif titles:
                title = titles[-1]
                for t in titles[:-1]:
                    structured_content.append({
                        "chapter": t,
                        "page": page_num + 1,
                        "content": ""
                    })
                lookahead = i + 1
                found = False
                while lookahead < len(blocks):
                    next_block = blocks["blocks"][lookahead]
                    next_content = extract_content_from_block(next_block, context_font, title_fonts)
                    next_titles = extract_title_from_block(next_block, title_fonts)
                    if next_content:
                        if current_section:
                            structured_content.append(current_section)
                        current_section = {
                            "chapter": title,
                            "page": page_num + 1,
                            "content": next_content
                        }
                        i = lookahead  # skip 到正文那一块
                        found = True
                        break
                    elif next_titles:
                        break
                    lookahead += 1

                if not found:
                    # 只有标题但后面没正文也没别的标题 → 先开空章节
                    if current_section:
                        structured_content.append(current_section)
                    current_section = {
                        "chapter": title,
                        "page": page_num + 1,
                        "content": ""
                    }

            i += 1

    # 最后一个章节
    if current_section:
        structured_content.append(current_section)

    return structured_content

def line_info(line):
    line_font_size = 0
    line_font = ""
    max_text_length = 0
    line_text = ""
    # 提取行文本和主要字体
    for span in line["spans"]:
        line_text += span["text"]
        line_font_size = max(line_font_size, span["size"])

        # 选择文本最多的span的字体作为行字体
        if len(span["text"].strip()) > max_text_length:
            max_text_length = len(span["text"].strip())
            line_font = span["font"]

    return line_font, line_font_size, line_text

def extract_title_from_block(block, title_fonts):
    """只提取标题，不处理内容"""
    titles = []
    if "lines" not in block:
        return titles

    lines = block["lines"]
    if not lines:
        return titles

    i = 0
    while i < min(4, len(lines)):
        line = lines[i]
        line_font, line_font_size, line_text = line_info(line)

        line_text = line_text.strip()
        if not line_text:
            continue

        current_font_tuple = (line_font, line_font_size)
        is_title = current_font_tuple in title_fonts and len(line_text) < 100

        if is_title:
            # 检查是否需要与下一行组合
            if re.match(r'^\d+(\.\d+)*\.?$', line_text) and i + 1 < len(lines):
                next_line = lines[i + 1]
                next_text = "".join([span["text"] for span in next_line["spans"]]).strip()
                if next_text and (next_text.isupper() or (next_text and next_text[0].isupper())):
                    chapter_num = line_text.strip().strip('.')
                    titles.append(f"{chapter_num}.{next_text}")
                    i += 2
                    continue
            titles.append(line_text)

        i += 1
    return titles

def extract_content_from_block(block, context_font, title_fonts):
    if "lines" not in block:
        return None
    # skip until content font is not dominant
    skip_lines = 0
    lines = block["lines"]
    for i, line in enumerate(lines):
        line_font, line_font_size, _ = line_info(line)
        if (line_font, line_font_size) == context_font:
            skip_lines = i
            break
        skip_lines = i

    content_lines = lines[skip_lines:]
    _, total_chars, rate = check_context_block(content_lines, context_font, title_fonts)
    # 如果非正文字体占比超过70%且总字符数超过100，则过滤
    if rate > 0.7 and total_chars > 100:
        # 只保留短文本（可能是公式、引用等）
        filtered_lines = []
        for line in content_lines:
            line_text = "".join([span["text"] for span in line["spans"]]).strip()

            # 保留短文本（小于50字符）或包含特殊符号的文本
            if (len(line_text) < 50 or
                re.search(r'[=≈≤≥∑∏∫±×÷√∞∂∇]', line_text) or
                re.search(r'\([^)]{1,20}\)', line_text) or  # 短括号内容
                re.search(r'\[[^\]]{1,20}\]', line_text)):  # 短方括号内容
                filtered_lines.append(line)

        content_lines = filtered_lines if filtered_lines else None

    # 直接转换为文本
    content_text = None
    if content_lines:
        content_text = "\n".join([
            "".join([span["text"] for span in line["spans"]])
            for line in content_lines
        ]).strip()

    return content_text

def check_context_block(lines, context_font, title_fonts):
    total_chars = 0
    non_context_font_chars = 0
    if not lines:
        return False, 0, 0.0

    for line in lines:
        for span in line.get("spans", []):
            span_text = span.get("text", "").strip()
            if not span_text:
                continue
            span_chars = len(span_text)
            total_chars += span_chars
            span_font = (span.get("font"), span.get("size"))
            if span_font != context_font and span_font not in title_fonts:
                non_context_font_chars += span_chars

    if total_chars == 0:
        return False, 0, 0.0

    non_context_ratio = non_context_font_chars / total_chars
    return non_context_ratio > 0.2 or total_chars < 20, total_chars, non_context_ratio

def check_consecutive_non_context(blocks, start, context_font, title_fonts, min_run=5):
    """This function is triggered only when block[start]'s main font is
        neither topic nor context
    """
    n = len(blocks['blocks'])
    if start >= n:
        return False, start

    def is_non_context_block(block):
        if "lines" not in block:
            return True  # 纯图片/空块视为非正文
        is_non_ctx, total, _ = check_context_block(block["lines"], context_font, title_fonts)
        # 使用 ratio_threshold 控制
        return is_non_ctx or (total == 0)

    if not is_non_context_block(blocks['blocks'][start]):
        return False, start

    end = start
    while end + 1 < n and is_non_context_block(blocks['blocks'][end + 1]):
        end += 1

    length = end - start + 1
    should_trim = length >= min_run
    return should_trim, end+1

if __name__ == '__main__':
    doc = pymupdf.open("/Users/xiangyuguan/Documents/projects/Reading_project/pdf_processor/utils/Cubic08.pdf")
    structured_data = analyze_text_structure(doc)
    with open("output.json", "w", encoding="utf-8") as out:
        json.dump(structured_data, out, ensure_ascii=False, indent=2)

    doc.close()
