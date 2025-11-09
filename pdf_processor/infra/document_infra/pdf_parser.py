import re
import pymupdf
import json
from collections import defaultdict


def clean_text(text: str) -> str:
    """Clean text for PostgreSQL compatibility.

    Removes:
    - NULL bytes (\x00) - PostgreSQL UTF-8 doesn't allow them
    - Other problematic control characters
    """
    if not text:
        return text

    # Remove NULL bytes and other problematic control characters
    # Keep newlines (\n), tabs (\t), and carriage returns (\r)
    text = text.replace('\x00', '')  # Remove NULL bytes

    # Remove other control characters except \n, \r, \t
    text = ''.join(char for char in text if ord(char) >= 32 or char in '\n\r\t')

    return text


def extract_chapter_number(chapter_title):
    """从章节标题中提取章节号

    Examples:
        "1.Introduction" -> "1"
        "2.1.Background" -> "2.1"
        "3.2.1 Methods" -> "3.2.1"
        "Introduction" -> ""
        "" -> ""
    """
    if not chapter_title:
        return ""

    # 匹配开头的章节号模式: 数字、点号、数字...
    match = re.match(r'^(\d+(?:\.\d+)*)\.?\s*', chapter_title)
    if match:
        return match.group(1)
    return ""


def analyze_text_structure(doc):
    import sys
    font_info = defaultdict(int)

    # 第一遍：收集所有字体大小和字体名
    print(f"DEBUG: Starting font collection, total pages: {len(doc)}", file=sys.stderr)
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
    print(f"DEBUG: Font collection complete. Context font: {context_font}", file=sys.stderr)

    # 第二遍：构建结构化内容
    structured_content = []
    current_section = None
    # 遍历每一页
    print(f"DEBUG: Starting page iteration for content extraction", file=sys.stderr)
    for page_num, page in enumerate(doc):
        print(f"DEBUG: Processing page {page_num + 1}/{len(doc)}", file=sys.stderr)
        # get blocks
        blocks = page.get_text("dict")
        print(f"DEBUG: Page {page_num + 1} has {len(blocks['blocks'])} blocks", file=sys.stderr)

        i = 0
        iteration_count = 0  # 防止无限循环
        max_iterations = len(blocks["blocks"]) * 2  # 最多迭代块数量的2倍

        while i < len(blocks["blocks"]):
            if iteration_count % 10 == 0:  # 每10次迭代打印一次
                print(f"DEBUG: Page {page_num + 1}, block {i}/{len(blocks['blocks'])}, iteration {iteration_count}", file=sys.stderr)
            # 防止无限循环
            iteration_count += 1
            if iteration_count > max_iterations:
                import sys
                print(f"WARNING: Infinite loop detected on page {page_num}, block {i}. Breaking.", file=sys.stderr)
                break

            block = blocks["blocks"][i]
            content = ""
            # get block and title if the block contains one
            titles = extract_title_from_block(block, title_fonts)
            stripped_block = extract_content_from_block(block, context_font, title_fonts)

            if "lines" not in block or not check_context_block(block["lines"], context_font, title_fonts)[0]:
                should_trim, re_start = check_consecutive_non_context(blocks, i, context_font, title_fonts)
                if should_trim:
                    # re_start is already the next position to process (end+1)
                    # Ensure we're making forward progress
                    if re_start <= i:
                        import sys
                        print(f"WARNING: check_consecutive_non_context returned re_start={re_start} <= i={i} on page {page_num}. Forcing i+1.", file=sys.stderr)
                        i += 1
                        continue
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
                            "chapter_num": extract_chapter_number(t),
                            "content": ""
                        })
                    current_section = {
                        "chapter": titles[-1],
                        "chapter_num": extract_chapter_number(titles[-1]),
                        "content": content
                    }
                else:
                    if current_section:
                        current_section["content"] += '\n' + content
                    else:
                        current_section = {
                            "chapter": "",
                            "chapter_num": "",
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
                        "chapter_num": extract_chapter_number(t),
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
                            "chapter_num": extract_chapter_number(title),
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
                        "chapter_num": extract_chapter_number(title),
                        "content": ""
                    }

            i += 1

        print(f"DEBUG: Completed page {page_num + 1}, total iterations: {iteration_count}", file=sys.stderr)

    # 最后一个章节
    if current_section:
        structured_content.append(current_section)

    print(f"DEBUG: Content extraction complete, total sections: {len(structured_content)}", file=sys.stderr)
    return structured_content

def line_info(line):
    line_font_size = 0
    line_font = ""
    max_text_length = 0
    line_text = ""
    # 提取行文本和主要字体
    for span in line["spans"]:
        # ✅ Clean text to remove NULL bytes
        line_text += clean_text(span["text"])
        line_font_size = max(line_font_size, span["size"])

        # 选择文本最多的span的字体作为行字体
        if len(span["text"].strip()) > max_text_length:
            max_text_length = len(span["text"].strip())
            line_font = span["font"]

    return line_font, line_font_size, line_text

def extract_title_from_block(block, title_fonts):
    """只提取标题，不处理内容"""
    import sys
    titles = []
    if "lines" not in block:
        return titles

    lines = block["lines"]
    if not lines:
        return titles

    i = 0
    iteration_count = 0
    max_iterations = len(lines) * 2

    while i < min(4, len(lines)):
        iteration_count += 1
        if iteration_count > max_iterations:
            print(f"WARNING: Infinite loop in extract_title_from_block, line {i}. Breaking.", file=sys.stderr)
            break
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
                # ✅ Clean text when combining lines
                next_text = "".join([clean_text(span["text"]) for span in next_line["spans"]]).strip()
                if next_text and (next_text.isupper() or (next_text and next_text[0].isupper())):
                    chapter_num = line_text.strip().strip('.')
                    titles.append(f"{chapter_num}.{next_text}")
                    i += 2
                    continue
            titles.append(line_text)

        i += 1
    return titles

def extract_content_from_block(block, context_font, title_fonts):
    import sys
    if "lines" not in block:
        return None
    # skip until content font is not dominant
    skip_lines = 0
    lines = block["lines"]

    if len(lines) > 1000:  # 防止异常大的 block
        print(f"WARNING: Block has {len(lines)} lines, may cause performance issues", file=sys.stderr)

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
            # ✅ Clean text when filtering
            line_text = "".join([clean_text(span["text"]) for span in line["spans"]]).strip()

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
        # ✅ Clean text when building final content
        content_text = "\n".join([
            "".join([clean_text(span["text"]) for span in line["spans"]])
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
    doc = pymupdf.open("/Users/xiangyuguan/Documents/MIT/6.5820//Cubic08.pdf")
    structured_data = analyze_text_structure(doc)
    with open("output.json", "w", encoding="utf-8") as out:
        json.dump(structured_data, out, ensure_ascii=False, indent=2)

    doc.close()
