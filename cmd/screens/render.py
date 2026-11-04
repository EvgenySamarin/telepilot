import json
import sys
from PIL import Image, ImageDraw, ImageFont, ImageFilter

def main():
    if len(sys.argv) < 3:
        print("Usage: render.py <title> <out_png>")
        sys.exit(1)

    title = sys.argv[1]
    out_png = sys.argv[2]

    raw_data = sys.stdin.read()
    if not raw_data:
        print("Error: empty input")
        sys.exit(1)

    lines_data = json.loads(raw_data)

    font_path = "/System/Library/Fonts/Menlo.ttc"
    font_size = 13
    font_reg = ImageFont.truetype(font_path, font_size, index=0)
    font_bld = ImageFont.truetype(font_path, font_size, index=1)
    title_font = ImageFont.truetype(font_path, 12, index=1)

    char_w = font_reg.getlength("A")
    char_h = 19.0

    num_rows = len(lines_data)
    num_cols = max(len(line) for line in lines_data) if lines_data else 125

    pad_x = 24.0
    pad_top = 44.0
    pad_bottom = 20.0

    term_w = num_cols * char_w
    term_h = num_rows * char_h
    win_w = int(term_w + pad_x * 2)
    win_h = int(term_h + pad_top + pad_bottom)

    margin = 25
    total_w = win_w + margin * 2
    total_h = win_h + margin * 2

    # Canvas (Tokyo Night dark background)
    img = Image.new("RGBA", (total_w, total_h), (13, 17, 23, 255))

    # Soft drop shadow
    shadow = Image.new("RGBA", (total_w, total_h), (0, 0, 0, 0))
    s_draw = ImageDraw.Draw(shadow)
    s_draw.rounded_rectangle(
        [margin, margin + 6, margin + win_w, margin + win_h + 6],
        radius=10,
        fill=(0, 0, 0, 160)
    )
    shadow = shadow.filter(ImageFilter.GaussianBlur(16))
    img.alpha_composite(shadow)

    draw = ImageDraw.Draw(img)
    ox, oy = margin, margin

    # Window container (#1a1b26) with border (#292e42)
    draw.rounded_rectangle([ox, oy, ox + win_w, oy + win_h], radius=10, fill=(26, 27, 38, 255), outline=(41, 46, 66, 255), width=1)

    # Titlebar (#16161e)
    draw.rounded_rectangle([ox, oy, ox + win_w, oy + int(pad_top)], radius=10, fill=(22, 22, 30, 255))
    draw.rectangle([ox, oy + int(pad_top) - 10, ox + win_w, oy + int(pad_top)], fill=(22, 22, 30, 255))

    # macOS window buttons
    draw.ellipse([ox + 18, oy + 15, ox + 30, oy + 27], fill=(247, 118, 142, 255))
    draw.ellipse([ox + 38, oy + 15, ox + 50, oy + 27], fill=(224, 175, 104, 255))
    draw.ellipse([ox + 58, oy + 15, ox + 70, oy + 27], fill=(158, 206, 106, 255))

    # Window title centered
    title_w = draw.textlength(title, font=title_font)
    draw.text((ox + (win_w - title_w) / 2, oy + 14), title, font=title_font, fill=(122, 162, 247, 255))

    # 1. Background highlights
    for y, line in enumerate(lines_data):
        cell_y = oy + pad_top + y * char_h
        for x, c in enumerate(line):
            bg = c.get("bg")
            if bg and bg.lower() not in ["#1a1b26", "#16161e"]:
                r = int(bg[1:3], 16)
                g = int(bg[3:5], 16)
                b = int(bg[5:7], 16)
                cell_x = ox + pad_x + x * char_w
                draw.rectangle([cell_x, cell_y, cell_x + char_w + 0.5, cell_y + char_h], fill=(r, g, b, 255))

    # 2. Text drawing
    for y, line in enumerate(lines_data):
        cell_y = oy + pad_top + y * char_h + 2.5
        run_text = ""
        run_x = 0
        run_fg = (192, 202, 245, 255)
        run_font = font_reg

        def flush_run():
            if run_text:
                draw.text((run_x, cell_y), run_text, font=run_font, fill=run_fg)

        for x, c in enumerate(line):
            ch = c.get("char", " ")
            fg = c.get("fg", "#c0caf5")
            bold = c.get("bold", False)

            # Boost low-contrast slate gray text so everything is crisp and readable
            if fg.lower() in ["#565f89", "#3b4261"]:
                if ch not in ["│", "─", "┌", "┐", "└", "┘", "║", "═", "╔", "╗", "╚", "╝", "├", "┤", "┬", "┴", "┼"]:
                    fg = "#7aa2f7"

            r = int(fg[1:3], 16)
            g = int(fg[3:5], 16)
            b = int(fg[5:7], 16)
            cur_fg = (r, g, b, 255)
            cur_font = font_bld if bold else font_reg
            cur_x = ox + pad_x + x * char_w

            if not run_text:
                run_text = ch
                run_x = cur_x
                run_fg = cur_fg
                run_font = cur_font
            elif cur_fg == run_fg and cur_font == run_font and abs((run_x + len(run_text) * char_w) - cur_x) < 0.1:
                run_text += ch
            else:
                flush_run()
                run_text = ch
                run_x = cur_x
                run_fg = cur_fg
                run_font = cur_font

        flush_run()

    img.save(out_png, "PNG")

if __name__ == "__main__":
    main()
