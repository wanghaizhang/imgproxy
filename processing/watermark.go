package processing

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/imgproxy/imgproxy/v4/imagedata"
	"github.com/imgproxy/imgproxy/v4/imath"
	"github.com/imgproxy/imgproxy/v4/options"
	"github.com/imgproxy/imgproxy/v4/options/keys"
	"github.com/imgproxy/imgproxy/v4/vips"
)

// watermarkPipeline constructs the watermark processing pipeline.
// This pipeline is applied to the watermark image.
func (p *Processor) watermarkPipeline() Pipeline {
	return Pipeline{
		p.vectorGuardScale,
		p.scaleOnLoad,
		p.colorspaceToProcessing,
		p.scale,
		p.rotateAndFlip,
		p.padding,
	}
}

func shouldReplicateWatermark(gt GravityType) bool {
	return gt == GravityReplicate
}

func (p *Processor) prepareWatermark(
	ctx context.Context,
	wm *vips.Image,
	wmData imagedata.ImageData,
	po ProcessingOptions,
	imgWidth, imgHeight int,
	offsetScale float64,
	framesCount int, //nolint:unparam
) error {
	if err := wm.Load(wmData, 1.0, 0, 1); err != nil {
		return err
	}

	_, _, err := p.checkImageSize(wm, wmData.Format(), po)
	if err != nil {
		return err
	}

	wmPo := p.NewProcessingOptions(options.New())
	wmPo.Set(keys.ResizingType, ResizeFit)
	wmPo.Set(keys.Dpr, 1)
	wmPo.Set(keys.Enlarge, true)
	wmPo.Set(keys.Format, wmData.Format())

	if scale := po.WatermarkScale(); scale > 0 {
		wmPo.Set(keys.Width, max(imath.ScaleToEven(imgWidth, scale), 1))
		wmPo.Set(keys.Height, max(imath.ScaleToEven(imgHeight, scale), 1))
	}

	shouldReplicate := shouldReplicateWatermark(po.WatermarkPosition())

	if shouldReplicate {
		offsetX := po.WatermarkXOffset()
		offsetY := po.WatermarkYOffset()

		var padX, padY int

		if math.Abs(offsetX) >= 1.0 {
			padX = imath.RoundToEven(offsetX * offsetScale)
		} else {
			padX = imath.ScaleToEven(imgWidth, offsetX)
		}

		if math.Abs(offsetY) >= 1.0 {
			padY = imath.RoundToEven(offsetY * offsetScale)
		} else {
			padY = imath.ScaleToEven(imgHeight, offsetY)
		}

		wmPo.Set(keys.PaddingLeft, padX/2)
		wmPo.Set(keys.PaddingRight, padX-padX/2)
		wmPo.Set(keys.PaddingTop, padY/2)
		wmPo.Set(keys.PaddingBottom, padY-padY/2)
	}

	if err := p.watermarkPipeline().Run(ctx, wm, wmPo, wmData); err != nil {
		return err
	}

	// We need to copy the image to ensure that it is in memory since we will
	// close it after watermark processing is done.
	if err := wm.CopyMemory(); err != nil {
		return err
	}

	if shouldReplicate {
		if err := wm.Replicate(imgWidth, imgHeight, true); err != nil {
			return err
		}
	}

	// We don't want any headers to be copied from the watermark to the image
	return wm.StripAll()
}

func (p *Processor) applyWatermark(
	ctx context.Context,
	img *vips.Image,
	po ProcessingOptions,
	offsetScale float64,
	framesCount int,
) error {
	if p.watermarkProvider == nil {
		return nil
	}

	wmData, _, err := p.watermarkProvider.Get(ctx, po.Options)
	if err != nil {
		return err
	}
	if wmData == nil {
		return nil
	}
	defer wmData.Close()

	wm := new(vips.Image)
	defer wm.Clear()

	width := img.Width()
	height := img.Height()
	frameHeight := height / framesCount

	if err := p.prepareWatermark(
		ctx, wm, wmData, po, width, frameHeight, offsetScale, framesCount,
	); err != nil {
		return err
	}

	opacity := po.WatermarkOpacity() * p.config.WatermarkOpacity

	position := po.WatermarkPosition()
	shouldReplicate := shouldReplicateWatermark(position)

	// If we replicated the watermark and need to apply it to an animated image,
	// it is faster to replicate the watermark to all the image and apply it single-pass
	if shouldReplicate && framesCount > 1 {
		if err := wm.Replicate(width, height, false); err != nil {
			return err
		}

		return img.ApplyWatermark(wm, 0, 0, opacity)
	}

	left, top := 0, 0
	wmWidth := wm.Width()
	wmHeight := wm.Height()

	if !shouldReplicate {
		gr := GravityOptions{
			Type: position,
			X:    po.WatermarkXOffset(),
			Y:    po.WatermarkYOffset(),
		}
		left, top = calcPosition(width, frameHeight, wmWidth, wmHeight, &gr, offsetScale, true)
	}

	if left >= width || top >= height || -left >= wmWidth || -top >= wmHeight {
		// Watermark is completely outside the image
		return nil
	}

	// if watermark is partially outside the image, it may partially be visible
	// on the next frame. We need to crop it vertically.
	// We don't care about horizontal overlap, as frames are stacked vertically
	if framesCount > 1 {
		cropTop := 0
		cropHeight := wmHeight

		if top < 0 {
			cropTop = -top
			cropHeight -= cropTop
			top = 0
		}

		if top+cropHeight > frameHeight {
			cropHeight = frameHeight - top
		}

		if cropTop > 0 || cropHeight < wmHeight {
			if err := wm.Crop(0, cropTop, wmWidth, cropHeight); err != nil {
				return err
			}
		}
	}

	for range framesCount {
		if err := img.ApplyWatermark(wm, left, top, opacity); err != nil {
			return err
		}
		top += frameHeight
	}

	return nil
}

const (
	// Reference DPI used to measure the text before scaling it to the target.
	textWatermarkRefDPI = 72
	// Safety cap so a very short text on a huge image can't allocate an
	// enormous text bitmap.
	textWatermarkMaxDPI = 3000
	// Gap kept between the text block and the image edges, in image pixels.
	textWatermarkPadding = 5
	// Gap kept between two lines of the text block, in image pixels.
	textWatermarkLineSpacing = 5
)

// textAlign is the horizontal alignment of the lines inside the text block.
type textAlign int

const (
	textAlignLeft textAlign = iota
	textAlignCenter
	textAlignRight
)

// textWatermarkAlign derives the line alignment from the watermark gravity:
// the text block is anchored to the image edge it was positioned against, so
// its lines follow that same edge. Anything centred stays centred.
func textWatermarkAlign(gt GravityType) textAlign {
	switch gt {
	case GravityEast, GravityNorthEast, GravitySouthEast:
		return textAlignRight
	case GravityWest, GravityNorthWest, GravitySouthWest:
		return textAlignLeft
	default:
		return textAlignCenter
	}
}

// offset returns the x offset of a line inside a block of blockWidth.
func (a textAlign) offset(blockWidth, lineWidth int) int {
	switch a {
	case textAlignRight:
		return blockWidth - lineWidth
	case textAlignCenter:
		return (blockWidth - lineWidth) / 2
	default:
		return 0
	}
}

// splitTextWatermarkLines splits the text into lines. Splitting happens before
// any markup is injected so that every line keeps balanced Pango markup.
func splitTextWatermarkLines(text string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			// Pango renders nothing for an empty line, which would collapse the
			// blank row. A single space keeps the line height.
			lines[i] = " "
		}
	}

	return lines
}

// measureTextWatermarkLine returns the pixel size of a line rendered at the
// reference DPI.
func measureTextWatermarkLine(line string) (int, int, error) {
	m := new(vips.Image)
	defer m.Clear()

	if err := m.ApplyTextWatermark(line, textWatermarkRefDPI); err != nil {
		return 0, 0, err
	}

	return m.Width(), m.Height(), nil
}

// textSize is the pixel size of a rendered piece of text.
type textSize struct {
	width  int
	height int
}

// textWatermarkDPI takes the sizes of every line measured at the reference DPI
// and returns the DPI that makes the widest line fill targetWidth, capped so
// the whole block can't be taller than maxHeight.
func textWatermarkDPI(sizes []textSize, targetWidth, maxHeight int) int {
	var maxWidth, totalHeight int

	for _, size := range sizes {
		maxWidth = max(maxWidth, size.width)
		totalHeight += size.height
	}

	if maxWidth <= 0 || totalHeight <= 0 {
		return 0
	}

	scale := float64(targetWidth) / float64(maxWidth)
	if maxHeight > 0 && float64(totalHeight)*scale > float64(maxHeight) {
		scale = float64(maxHeight) / float64(totalHeight)
	}

	return min(max(int(math.Round(textWatermarkRefDPI*scale)), 1), textWatermarkMaxDPI)
}

// insetWatermarkPosition pulls the text block back inside the padded area so the
// gap survives explicit offsets and rounding.
func insetWatermarkPosition(
	left, top, imgWidth, imgHeight, blockWidth, blockHeight, pad int,
) (int, int) {
	left = max(min(left, imgWidth-pad-blockWidth), pad)
	top = max(min(top, imgHeight-pad-blockHeight), pad)

	return left, top
}

// textWatermarkBlock is a stack of rendered text lines.
type textWatermarkBlock struct {
	lines  []*vips.Image
	width  int // width of the widest line
	height int // sum of all line heights
}

func (b *textWatermarkBlock) clear() {
	for _, line := range b.lines {
		line.Clear()
	}
	b.lines = nil
}

// renderTextWatermarkBlock renders every line at dpi and measures the block
// they form.
func renderTextWatermarkBlock(lines []string, dpi int) (*textWatermarkBlock, error) {
	b := &textWatermarkBlock{}

	for i, line := range lines {
		img := new(vips.Image)

		if err := img.ApplyTextWatermark(line, dpi); err != nil {
			img.Clear()
			b.clear()

			return nil, err
		}

		b.lines = append(b.lines, img)
		b.width = max(b.width, img.Width())
		b.height += img.Height()
		if i < len(lines)-1 {
			b.height += textWatermarkLineSpacing
		}
	}

	return b, nil
}

// renderTextWatermark renders a Pango-markup text watermark onto the image.
// The text is Pango-markup encoded (e.g. "<span foreground='red' weight='bold'>
// Hello</span>"). Opacity, position, and scale follow the same conventions as
// the image watermark. This uses libvips' vips_text(), which is available in
// the open-source libvips — no imgproxy Pro feature required.
func (p *Processor) renderTextWatermark(
	_ context.Context,
	img *vips.Image,
	po ProcessingOptions,
	offsetScale float64,
	opacity float64,
) error {
	text := po.WatermarkText()
	if text == "" {
		return nil
	}

	lines := splitTextWatermarkLines(text)

	// Pango renders the text in black unless the markup says otherwise, so an
	// explicit watermark.text_color has to be injected into the markup. The
	// span is applied per line: wrapping the whole text would leave it
	// unbalanced after the text is split into lines.
	if po.Has(keys.WatermarkTextColor) {
		c := po.WatermarkTextColor()
		span := fmt.Sprintf(`<span foreground="#%02X%02X%02X">%%s</span>`, c.R, c.G, c.B)

		for i, line := range lines {
			lines[i] = fmt.Sprintf(span, line)
		}
	}

	width := img.Width()
	height := img.Height()

	// Keep a small gap between the text and the image edges. Images narrower
	// than twice the gap just lose the gap instead of failing.
	pad := textWatermarkPadding
	if width <= 2*pad || height <= 2*pad {
		pad = 0
	}
	innerWidth := width - 2*pad
	innerHeight := height - 2*pad

	// Text width follows the text scale option; when unset, fill the image
	// (minus the gap on both sides).
	textWidth := innerWidth
	if textScale := po.WatermarkTextScale(); textScale > 0 {
		textWidth = min(max(imath.ScaleToEven(width, textScale), 1), innerWidth)
	}

	// Adaptive font size: measure the glyphs at a reference DPI, then render
	// again at the DPI that makes the widest line fill textWidth. Unlike a
	// character-count estimate this is immune to Pango markup, mixed alphabets
	// (CJK vs Latin) and explicit colors.
	sizes := make([]textSize, len(lines))
	for i, line := range lines {
		w, h, err := measureTextWatermarkLine(line)
		if err != nil {
			return err
		}

		sizes[i] = textSize{width: w, height: h}
	}

	dpi := textWatermarkDPI(sizes, textWidth, innerHeight)
	if dpi == 0 {
		// The text rendered nothing (e.g. empty Pango markup).
		return nil
	}

	block, err := renderTextWatermarkBlock(lines, dpi)
	if err != nil {
		return err
	}
	// Closure on purpose: `block` is reassigned when the DPI is shrunk below,
	// and a plain `defer block.clear()` would keep clearing the first block.
	defer func() { block.clear() }()

	// Line heights are measured separately and then summed, so rounding can
	// make the block a couple of pixels larger than the space it has to fit in.
	// Shrink the DPI until it fits and the gap is preserved.
	for attempt := 0; (block.width > innerWidth || block.height > innerHeight) && dpi > 1 && attempt < 4; attempt++ {
		scale := math.Min(
			float64(innerWidth)/float64(max(block.width, 1)),
			float64(innerHeight)/float64(max(block.height, 1)),
		)

		next := max(int(math.Floor(float64(dpi)*scale)), 1)
		if next >= dpi {
			next = dpi - 1
		}

		block.clear()

		block, err = renderTextWatermarkBlock(lines, next)
		if err != nil {
			return err
		}

		dpi = next
	}

	// Position the text block using the same gravity logic as the image
	// watermark, then pull it back inside the padded area so the gap on every
	// edge survives explicit offsets.
	position := po.WatermarkPosition()
	align := textWatermarkAlign(position)

	left, top := pad, pad
	if !shouldReplicateWatermark(position) {
		gr := GravityOptions{
			Type: position,
			X:    po.WatermarkXOffset(),
			Y:    po.WatermarkYOffset(),
		}
		left, top = calcPosition(width, height, block.width, block.height, &gr, offsetScale, true)
		left, top = insetWatermarkPosition(left, top, width, height, block.width, block.height, pad)
	}

	// Lines are aligned to the edge the block is anchored to, so a short line
	// above a long one still ends flush with the long one.
	y := top
	for i, line := range block.lines {
		x := left + align.offset(block.width, line.Width())

		// img.ApplyWatermark applies the opacity to the watermark alpha channel.
		if err := img.ApplyWatermark(line, x, y, opacity); err != nil {
			return err
		}

		y += line.Height()
		if i < len(block.lines)-1 {
			y += textWatermarkLineSpacing
		}
	}

	return nil
}

func (p *Processor) watermark(c *Context) error {
	// Text watermark takes precedence when watermark_text is set.
	if text := c.PO.WatermarkText(); text != "" {
		opacity := c.PO.WatermarkOpacity()
		if opacity == 0 {
			// Default opacity when not explicitly set. Kept local on purpose:
			// writing it back into the shared options would leak into the
			// per-frame processing of animated images.
			opacity = 1.0
		}

		return p.renderTextWatermark(c.Ctx, c.Img, c.PO, c.DprScale, opacity)
	}

	if p.watermarkProvider == nil || c.PO.WatermarkOpacity() == 0 {
		return nil
	}

	return p.applyWatermark(c.Ctx, c.Img, c.PO, c.DprScale, 1)
}
