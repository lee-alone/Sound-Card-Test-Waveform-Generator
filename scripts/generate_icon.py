"""
Icon generator for Sound Card Test Waveform Generator.
Generates modern pro-audio spectrum analyzer icons across all standard Windows resolutions.
"""

import math
import os
from PIL import Image, ImageDraw, ImageFilter


def create_spectrum_icon(size: int = 1024) -> Image.Image:
    # Work on a high-res canvas (1024x1024) with RGBA
    canvas = Image.new("RGBA", (size, size), (0, 0, 0, 0))

    # Scale factor relative to 1024 base
    s = size / 1024.0

    # 1. Background Squircle (Rounded Rectangle)
    pad = int(80 * s)
    radius = int(190 * s)
    bg_box = [pad, pad, size - pad, size - pad]

    # Create background mask with rounded corners
    bg_mask = Image.new("L", (size, size), 0)
    mask_draw = ImageDraw.Draw(bg_mask)
    mask_draw.rounded_rectangle(bg_box, radius=radius, fill=255)

    # Background gradient: Dark industrial slate navy
    bg_img = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    for y in range(size):
        ratio = y / size
        # Top: #121826 (18, 24, 38) -> Bottom: #090c13 (9, 12, 19)
        r = int(18 * (1 - ratio) + 9 * ratio)
        g = int(24 * (1 - ratio) + 12 * ratio)
        b = int(38 * (1 - ratio) + 19 * ratio)
        line = Image.new("RGBA", (size, 1), (r, g, b, 255))
        bg_img.paste(line, (0, y))

    # Apply rounded squircle mask
    bg_composite = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    bg_composite.paste(bg_img, (0, 0), bg_mask)

    # Add soft drop shadow behind the squircle
    shadow_mask = bg_mask.filter(ImageFilter.GaussianBlur(int(24 * s)))
    shadow = Image.new("RGBA", (size, size), (0, 0, 0, 140))
    canvas.paste(shadow, (0, int(16 * s)), shadow_mask)

    # Paste background
    canvas.alpha_composite(bg_composite)

    # 2. Subtle Border Highlight
    border_layer = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    border_draw = ImageDraw.Draw(border_layer)
    border_width = max(2, int(4 * s))
    border_draw.rounded_rectangle(bg_box, radius=radius, outline=(56, 189, 248, 70), width=border_width)
    canvas.alpha_composite(border_layer)

    # 3. Ambient Analyzer Graticule (faint horizontal dB grid lines)
    grid_layer = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    grid_draw = ImageDraw.Draw(grid_layer)
    grid_x0 = int(170 * s)
    grid_x1 = int(854 * s)
    grid_ys = [int(y * s) for y in (310, 430, 550, 670, 770)]
    for y in grid_ys[:-1]:
        grid_draw.line([(grid_x0, y), (grid_x1, y)], fill=(255, 255, 255, 18), width=max(1, int(2 * s)))
    # Baseline
    grid_draw.line([(grid_x0, grid_ys[-1]), (grid_x1, grid_ys[-1])], fill=(56, 189, 248, 90), width=max(2, int(3 * s)))
    canvas.alpha_composite(grid_layer)

    # 4. Spectrum Analyzer Bars (7 bands: Bass -> Mid -> Treble)
    baseline_y = int(770 * s)
    num_bars = 7
    bar_w = int(64 * s)
    bar_gap = int(36 * s)
    total_w = num_bars * bar_w + (num_bars - 1) * bar_gap
    start_x = (size - total_w) // 2

    # Heights relative to 1024 base
    bar_heights = [int(h * s) for h in (260, 390, 500, 560, 460, 360, 230)]
    peak_offset = int(22 * s)
    peak_h = max(3, int(12 * s))

    # Glow layer for bars
    glow_layer = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    glow_draw = ImageDraw.Draw(glow_layer)

    # Bar drawing layer
    bar_layer = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    bar_draw = ImageDraw.Draw(bar_layer)

    def get_spectrum_color(y_pos: int, base_y: int, max_y: int):
        norm = max(0.0, min(1.0, (base_y - y_pos) / (base_y - max_y)))
        if norm < 0.45:
            # Cyan to Emerald: (6, 182, 212) -> (16, 185, 129)
            t = norm / 0.45
            r = int(6 * (1 - t) + 16 * t)
            g = int(182 * (1 - t) + 185 * t)
            b = int(212 * (1 - t) + 129 * t)
        elif norm < 0.8:
            # Emerald to Amber: (16, 185, 129) -> (245, 158, 11)
            t = (norm - 0.45) / 0.35
            r = int(16 * (1 - t) + 245 * t)
            g = int(185 * (1 - t) + 158 * t)
            b = int(129 * (1 - t) + 11 * t)
        else:
            # Amber to Coral Red: (245, 158, 11) -> (239, 68, 68)
            t = (norm - 0.8) / 0.2
            r = int(245 * (1 - t) + 239 * t)
            g = int(158 * (1 - t) + 68 * t)
            b = int(11 * (1 - t) + 68 * t)
        return (r, g, b, 245)

    max_peak_y = baseline_y - max(bar_heights)

    for i in range(num_bars):
        bx0 = start_x + i * (bar_w + bar_gap)
        bx1 = bx0 + bar_w
        bh = bar_heights[i]
        by0 = baseline_y - bh
        by1 = baseline_y

        bar_radius = bar_w // 2
        single_mask = Image.new("L", (size, size), 0)
        s_draw = ImageDraw.Draw(single_mask)
        s_draw.rounded_rectangle([bx0, by0, bx1, by1], radius=bar_radius, fill=255)

        bar_tex = Image.new("RGBA", (size, size), (0, 0, 0, 0))
        for y in range(by0, by1 + 1):
            color = get_spectrum_color(y, baseline_y, max_peak_y)
            line = Image.new("RGBA", (bar_w, 1), color)
            bar_tex.paste(line, (bx0, y))

        bar_composite = Image.new("RGBA", (size, size), (0, 0, 0, 0))
        bar_composite.paste(bar_tex, (0, 0), single_mask)
        bar_layer.alpha_composite(bar_composite)

        glow_draw.rounded_rectangle([bx0, by0, bx1, by1], radius=bar_radius, fill=(6, 182, 212, 100))

        py0 = by0 - peak_offset - peak_h
        py1 = by0 - peak_offset
        peak_color = get_spectrum_color(py0, baseline_y, max_peak_y)
        peak_color = (min(255, peak_color[0] + 30), min(255, peak_color[1] + 30), min(255, peak_color[2] + 40), 255)
        bar_draw.rounded_rectangle([bx0, py0, bx1, py1], radius=peak_h // 2, fill=peak_color)
        glow_draw.rounded_rectangle([bx0, py0, bx1, py1], radius=peak_h // 2, fill=(255, 255, 255, 120))

    glow_blurred = glow_layer.filter(ImageFilter.GaussianBlur(int(14 * s)))
    canvas.alpha_composite(glow_blurred)
    canvas.alpha_composite(bar_layer)

    # 5. Glowing Test Waveform Curve (Analog Sine Wave crossing the spectrum)
    wave_layer = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    wave_draw = ImageDraw.Draw(wave_layer)
    wave_points = []
    w_x0 = int(140 * s)
    w_x1 = int(884 * s)
    w_center_y = int(520 * s)
    w_amp = int(130 * s)
    w_freq = 2.4 * math.pi / (w_x1 - w_x0)

    for x in range(w_x0, w_x1 + 1):
        y = w_center_y - int(w_amp * math.sin((x - w_x0) * w_freq))
        wave_points.append((x, y))

    wave_glow = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    wg_draw = ImageDraw.Draw(wave_glow)
    wg_draw.line(wave_points, fill=(56, 189, 248, 160), width=max(4, int(14 * s)), joint="curve")
    canvas.alpha_composite(wave_glow.filter(ImageFilter.GaussianBlur(int(8 * s))))

    wave_draw.line(wave_points, fill=(224, 242, 254, 230), width=max(2, int(4 * s)), joint="curve")
    wave_draw.line(wave_points, fill=(255, 255, 255, 255), width=max(1, int(2 * s)), joint="curve")
    canvas.alpha_composite(wave_layer)

    return canvas


def generate_all_icons():
    os.makedirs("assets/icon", exist_ok=True)
    os.makedirs("internal/ui/assets", exist_ok=True)

    base_img = create_spectrum_icon(1024)

    # Standard icon sizes
    sizes = [256, 128, 64, 48, 32, 24, 16]
    images_by_size = {}

    for sz in sizes:
        resized = base_img.resize((sz, sz), Image.Resampling.LANCZOS)
        images_by_size[sz] = resized

    # Save master ICO
    ico_path = "assets/icon/app.ico"
    images_by_size[256].save(
        ico_path,
        format="ICO",
        sizes=[(sz, sz) for sz in sizes],
        append_images=[images_by_size[sz] for sz in sizes[1:]],
    )
    print(f"Saved multi-resolution ICO: {ico_path} with sizes: {sizes}")

    # Save assets/icon/app.png (256x256)
    png_path = "assets/icon/app.png"
    images_by_size[256].save(png_path, format="PNG")
    print(f"Saved master PNG: {png_path}")

    # Save embedded PNGs for internal/ui/assets
    embedded_sizes = [16, 32, 48, 64, 256]
    for sz in embedded_sizes:
        target = f"internal/ui/assets/icon_{sz}.png"
        images_by_size[sz].save(target, format="PNG")
        print(f"Saved runtime UI asset: {target}")


if __name__ == "__main__":
    generate_all_icons()
