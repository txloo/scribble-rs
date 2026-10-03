#!/usr/bin/perl
#
# Regenerates the placeholder media that replaced the assets which upstream
# scribble-rs/scribble.rs explicitly excludes from its BSD-3 grant (its logo,
# background and favicon -- see README "Credits").
#
# Everything produced here is drawn from scratch by this script: no third-party
# artwork is copied, so the whole set is ours to license. Colours are taken from
# resources/root.css so the placeholders sit correctly on the existing theme.
#
# Usage:  perl tools/placeholder-assets/generate.pl
# Writes: internal/frontend/resources/{logo.svg,logo.png,background.png,
#                                    favicon.svg,favicon_16.png,
#                                    favicon_32.png,favicon_96.png}

use strict;
use warnings;
use Compress::Zlib qw(compress crc32);

my $RES = 'internal/frontend/resources';

# Palette lifted from internal/frontend/resources/root.css
my %C = (
    blue_deep => [46, 46, 174],    # rgba(46, 46, 175, 1)   gradient start
    blue_mid  => [37, 79, 191],    # rgba(37, 79, 191, 1)   gradient 42%
    blue_lite => [0, 170, 255],    # rgba(0, 170, 255, 1)   gradient end
    white     => [255, 255, 255],
);

# The mark: a scribble stroke in a normalised 0..1 box.
my @SCRIBBLE = (
    [0.10, 0.72], [0.26, 0.30], [0.42, 0.66],
    [0.58, 0.26], [0.74, 0.62], [0.90, 0.34],
);

# ---------------------------------------------------------------------------
# Canvas. The pixel buffer is held as a scalar ref so that per-pixel writes use
# in-place 4-arg substr instead of copying the whole buffer.
# ---------------------------------------------------------------------------

sub new_canvas {
    my ($w, $h) = @_;
    my $px = "\0" x ($w * $h * 4);
    return { w => $w, h => $h, px => \$px };
}

sub blend {
    my ($cv, $x, $y, $rgb, $alpha) = @_;
    return if $x < 0 || $y < 0 || $x >= $cv->{w} || $y >= $cv->{h};
    return if $alpha <= 0;
    $alpha = 1 if $alpha > 1;
    my $px = $cv->{px};
    my $off = ($y * $cv->{w} + $x) * 4;
    my @dst = unpack 'C4', substr($$px, $off, 4);
    my $ia  = 1 - $alpha;
    substr($$px, $off, 4) = pack 'C4',
        int($rgb->[0] * $alpha + $dst[0] * $ia + 0.5),
        int($rgb->[1] * $alpha + $dst[1] * $ia + 0.5),
        int($rgb->[2] * $alpha + $dst[2] * $ia + 0.5),
        int($alpha * 255 + $dst[3] * $ia + 0.5);
}

sub set_px {
    my ($cv, $x, $y, $rgb, $a) = @_;
    my $off = ($y * $cv->{w} + $x) * 4;
    substr(${ $cv->{px} }, $off, 4) = pack 'C4', @$rgb, $a;
}

# Distance from a point to a segment, for antialiased thick strokes.
sub _seg_dist {
    my ($px, $py, $ax, $ay, $bx, $by) = @_;
    my $dx = $bx - $ax;
    my $dy = $by - $ay;
    my $len2 = $dx * $dx + $dy * $dy;
    my $t = $len2 == 0 ? 0 : (($px - $ax) * $dx + ($py - $ay) * $dy) / $len2;
    $t = 0 if $t < 0;
    $t = 1 if $t > 1;
    my $cx = $ax + $t * $dx;
    my $cy = $ay + $t * $dy;
    return sqrt(($px - $cx) ** 2 + ($py - $cy) ** 2);
}

sub stroke_path {
    my ($cv, $pts, $width, $rgb, $alpha) = @_;
    my $half = $width / 2;
    my $pad  = $half + 1;
    for my $i (0 .. $#$pts - 1) {
        my ($ax, $ay) = @{ $pts->[$i] };
        my ($bx, $by) = @{ $pts->[$i + 1] };
        my $x0 = $ax < $bx ? $ax : $bx;
        my $x1 = $ax < $bx ? $bx : $ax;
        my $y0 = $ay < $by ? $ay : $by;
        my $y1 = $ay < $by ? $by : $ay;
        for my $y (int($y0 - $pad) .. int($y1 + $pad)) {
            for my $x (int($x0 - $pad) .. int($x1 + $pad)) {
                next if $x < 0 || $y < 0 || $x >= $cv->{w} || $y >= $cv->{h};
                my $cov = $half + 0.5 - _seg_dist($x + 0.5, $y + 0.5, $ax, $ay, $bx, $by);
                next if $cov <= 0;
                blend($cv, $x, $y, $rgb, $cov > 1 ? 1 : $cov);
            }
        }
    }
}

sub round_rect {
    my ($cv, $x0, $y0, $x1, $y1, $r, $rgb, $alpha) = @_;
    for my $y (int($y0) .. int($y1)) {
        for my $x (int($x0) .. int($x1)) {
            next if $x < 0 || $y < 0 || $x >= $cv->{w} || $y >= $cv->{h};
            my $px = $x + 0.5;
            my $py = $y + 0.5;
            # Either we sit in the cross-shaped inner area, or inside a corner
            # quadrant where the corner circle decides.
            my $inside = ($px >= $x0 + $r && $px <= $x1 - $r)
                || ($py >= $y0 + $r && $py <= $y1 - $r);
            if (!$inside) {
                my $cx = $px < $x0 + $r ? $x0 + $r : $x1 - $r;
                my $cy = $py < $y0 + $r ? $y0 + $r : $y1 - $r;
                $inside = sqrt(($px - $cx) ** 2 + ($py - $cy) ** 2) <= $r;
            }
            blend($cv, $x, $y, $rgb, $alpha) if $inside;
        }
    }
}

# ---------------------------------------------------------------------------
# Minimal 5x7 bitmap font -- only the glyphs the wordmark needs.
# ---------------------------------------------------------------------------

my %FONT = (
    'S' => ['.###.', '#...#', '#....', '.###.', '....#', '#...#', '.###.'],
    'c' => ['.###.', '#...#', '#....', '#....', '#....', '#...#', '.###.'],
    'r' => ['.##..', '#..#.', '#....', '.##..', '#.#..', '#..#.', '#...#'],
    'i' => ['..#..', '.##..', '..#..', '..#..', '..#..', '..#..', '.###.'],
    'b' => ['##...', '#.#..', '#..#.', '#...#', '#...#', '#..#.', '##...'],
    'l' => ['.###.', '..#..', '..#..', '..#..', '..#..', '..#..', '.###.'],
    'e' => ['.###.', '#...#', '#....', '####.', '#....', '#...#', '.###.'],
    's' => ['.####', '#....', '#....', '.###.', '....#', '....#', '####.'],
    '.' => ['.....', '.....', '.....', '.....', '.....', '.##..', '.##..'],
);

sub draw_text {
    my ($cv, $text, $x, $y, $scale, $rgb, $alpha, $tracking) = @_;
    $tracking //= 1;
    my $cx = $x;
    for my $ch (split //, $text) {
        my $glyph = $FONT{$ch} or next;
        for my $row (0 .. 6) {
            for my $col (0 .. 4) {
                next if substr($glyph->[$row], $col, 1) ne '#';
                for my $dy (0 .. $scale - 1) {
                    my $sy = $y + $row * $scale + $dy;
                    next if $sy < 0 || $sy >= $cv->{h};
                    for my $dx (0 .. $scale - 1) {
                        blend($cv, $cx + $col * $scale + $dx, $sy, $rgb, $alpha);
                    }
                }
            }
        }
        $cx += (5 + $tracking) * $scale;
    }
}

# ---------------------------------------------------------------------------
# PNG (8-bit RGBA, filter type 0) and file output
# ---------------------------------------------------------------------------

sub _chunk {
    my ($type, $data) = @_;
    return pack('N', length $data) . $type . $data
        . pack('N', crc32($type . $data));
}

sub write_png {
    my ($path, $cv) = @_;
    my $raw = '';
    for my $y (0 .. $cv->{h} - 1) {
        $raw .= "\0" . substr(${ $cv->{px} }, $y * $cv->{w} * 4, $cv->{w} * 4);
    }
    my $png = "\x89PNG\r\n\x1a\n"
        . _chunk('IHDR', pack('NNCCCCC', $cv->{w}, $cv->{h}, 8, 6, 0, 0, 0))
        . _chunk('IDAT', compress($raw))
        . _chunk('IEND', '');
    open my $fh, '>', $path or die "cannot write $path: $!";
    binmode $fh;
    print $fh $png;
    close $fh;
    printf "wrote %-42s %4dx%-4d\n", $path, $cv->{w}, $cv->{h};
}

sub write_text_file {
    my ($path, $content) = @_;
    open my $fh, '>', $path or die "cannot write $path: $!";
    print $fh $content;
    close $fh;
    printf "wrote %-42s %7d bytes\n", $path, length $content;
}

# Same two-stop gradient as body in root.css, so the og:image matches the page.
sub gradient_at {
    my ($t) = @_;
    my ($a, $b) = ($C{blue_deep}, $C{blue_lite});
    return [ map { int($_ + 0.5) }
        map { $a->[$_] + ($b->[$_] - $a->[$_]) * $t } 0 .. 2 ];
}

# ---------------------------------------------------------------------------
# Assets
# ---------------------------------------------------------------------------

# The mark as absolute points, optionally truncated to fewer segments.
sub mark_points {
    my ($x0, $y0, $size, $count) = @_;
    return [ map { [ $x0 + $_->[0] * $size, $y0 + $_->[1] * $size ] }
        @SCRIBBLE[ 0 .. $count ] ];
}

sub gen_favicon_png {
    my ($size) = @_;
    my $cv = new_canvas($size, $size);
    my $inset = $size >= 32 ? int($size * 0.06) : 0;
    round_rect($cv, $inset, $inset, $size - 1 - $inset, $size - 1 - $inset,
        $size * 0.22, $C{white}, 1);
    # A 16px tab has no room for six segments; use a shorter, bolder mark.
    my $pad = $size * 0.18;
    stroke_path($cv, mark_points($pad, $pad, $size - 2 * $pad,
            $size >= 32 ? 5 : 3), $size * 0.13, $C{blue_mid}, 1);
    write_png("$RES/favicon_$size.png", $cv);
}

sub gen_background_png {
    # 400x400, seamlessly tiling to match `background-size: 400px 400px` in
    # root.css. Faint white dots, because root.css composites this over the
    # body gradient at opacity 0.5.
    my ($size) = @_;
    my $cv = new_canvas($size, $size);
    my $r = $size / 46;
    for my $gy (0 .. 3) {
        for my $gx (0 .. 3) {
            my $cx = ($gx + 0.5) * $size / 4;
            my $cy = ($gy + 0.5) * $size / 4;
            for my $dy (-$r .. $r) {
                for my $dx (-$r .. $r) {
                    my $d = sqrt($dx * $dx + $dy * $dy);
                    next if $d > $r;
                    # Wrapped coordinates keep the tile seamless.
                    blend($cv,
                        (int($cx + $dx) % $size + $size) % $size,
                        (int($cy + $dy) % $size + $size) % $size,
                        $C{white}, (1 - $d / $r) * 0.5);
                }
            }
        }
    }
    write_png("$RES/background.png", $cv);
}

sub gen_logo_png {
    # og:image / twitter:image, referenced from templates/index.html. Needs its
    # own backdrop: social scrapers render it outside the page.
    my ($w, $h) = @_;
    my $cv = new_canvas($w, $h);
    for my $x (0 .. $w - 1) {
        my $col = gradient_at($x / ($w - 1));
        for my $y (0 .. $h - 1) {
            set_px($cv, $x, $y, $col, 255);
        }
    }
    # Lay the lockup out from the canvas size instead of fixed numbers, so the
    # mark and the wordmark always fit side by side and stay centred.
    my $word   = 'Scribble.rs';
    my $margin = $w * 0.09;
    my $mark   = $h * 0.25;
    my $gap    = $mark * 0.30;
    # Width of one bitmap string: 5px per glyph plus 1px tracking per join.
    my $adv    = 5 * length($word) + (length($word) - 1);
    my $scale  = int(($w - 2 * $margin - $mark - $gap) / $adv);
    die "wordmark does not fit in ${w}x${h}\n" if $scale < 1;
    my $lockup = $mark + $gap + $adv * $scale;
    my $mx     = int(($w - $lockup) / 2);
    stroke_path($cv, mark_points($mx, ($h - $mark) / 2, $mark, 5),
        $mark * 0.085, $C{white}, 1);
    draw_text($cv, $word, $mx + $mark + $gap,
        int(($h - 7 * $scale) / 2), $scale, $C{white}, 1);
    printf "  logo.png lockup: mark=%d gap=%d scale=%d -> %dpx wide\n",
        $mark, $gap, $scale, $lockup;
    write_png("$RES/logo.png", $cv);
}

sub rgb_str { return "rgb($_[0][0],$_[0][1],$_[0][2])" }

sub polyline_attr {
    my ($x0, $y0, $size, $count) = @_;
    return join ' ', map { sprintf '%.1f,%.1f', @$_ } @{ mark_points($x0, $y0, $size, $count) };
}

sub gen_favicon_svg {
    # 76x76 box; the mark is 60px, inset 8px so the stroke stays inside.
    my $pts = polyline_attr(8, 8, 60, 5);
    write_text_file("$RES/favicon.svg", <<"SVG");
<?xml version="1.0" encoding="UTF-8"?>
<!-- Placeholder favicon. Original artwork produced by
     tools/placeholder-assets/generate.pl. Replaces the upstream favicon, which
     is excluded from scribble.rs' BSD-3 grant. -->
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 76 76" width="76" height="76">
  <rect x="1" y="1" width="74" height="74" rx="17" fill="@{[ rgb_str($C{white}) ]}"/>
  <polyline points="$pts" fill="none" stroke="@{[ rgb_str($C{blue_mid}) ]}"
    stroke-width="8" stroke-linecap="round" stroke-linejoin="round"/>
</svg>
SVG
}

sub gen_logo_svg {
    # 1133x208 box; the mark is 150px tall and vertically centred (208-150)/2.
    my $pts = polyline_attr(14, 29, 150, 5);
    write_text_file("$RES/logo.svg", <<"SVG");
<?xml version="1.0" encoding="UTF-8"?>
<!-- Placeholder logo. Original artwork produced by
     tools/placeholder-assets/generate.pl. Replaces the upstream logo, which is
     excluded from scribble.rs' BSD-3 grant. -->
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1133 208" width="1133" height="208">
  <polyline points="$pts" fill="none" stroke="@{[ rgb_str($C{white}) ]}"
    stroke-width="13" stroke-linecap="round" stroke-linejoin="round"/>
  <text x="196" y="148" font-family="Inter, system-ui, Helvetica, Arial, sans-serif"
    font-size="112" font-weight="700" fill="@{[ rgb_str($C{white}) ]}">Scribble.rs</text>
</svg>
SVG
}

gen_favicon_svg();
gen_logo_svg();
gen_favicon_png(16);
gen_favicon_png(32);
gen_favicon_png(96);
gen_background_png(400);
gen_logo_png(1600, 800);
