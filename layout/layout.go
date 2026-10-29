package layout

// Mode says what a size offered to a node means.
type Mode uint8

const (
	Undefined Mode = iota // no limit: size to content
	Exactly               // the node must be this size
	AtMost                // size to content, but no larger than this
)

// MeasureFunc sizes a leaf (text, an image, a native control) within the
// space offered. A width or height whose mode is Undefined is not a limit.
type MeasureFunc func(width float32, wMode Mode, height float32, hMode Mode) (w, h float32)

// Node is one box in the layout tree. Set Style, Children and, for leaves,
// Measure; call Calculate on the root; read the results.
type Node struct {
	Style    Style
	Measure  MeasureFunc
	Children []*Node

	// Results. X and Y are the border box's offset from the parent's border
	// box; W and H its size. ContentW and ContentH are the extent of the
	// children (padding included) — a scroll view's content size.
	X, Y, W, H         float32
	ContentW, ContentH float32

	pass  uint32
	cache []cacheEntry
}

type cacheEntry struct {
	availW, availH, ownerW, ownerH float32
	wMode, hMode                   Mode
	free                           axes
	w, h                           float32
}

// axes is a set of axes; a node measured with an axis "free" ignores its own
// min/max size on it, as CSS does when finding an item's flex base size.
type axes uint8

const (
	freeW axes = 1 << iota
	freeH
)

var passCounter uint32

// Calculate lays out the tree rooted at n within width × height (the
// viewport). An undefined dimension (NaN) sizes the root to its content.
func (n *Node) Calculate(width, height float32) {
	passCounter++
	p := &pass{id: passCounter}
	s := &n.Style
	wMode, hMode := Exactly, Exactly
	w, h := s.Width.resolve(width), s.Height.resolve(height)
	if defined(w) {
		width = clamp(w, s.MinWidth.resolve(width), s.MaxWidth.resolve(width))
	} else if !defined(width) {
		wMode = Undefined
	}
	if defined(h) {
		height = clamp(h, s.MinHeight.resolve(height), s.MaxHeight.resolve(height))
	} else if !defined(height) {
		hMode = Undefined
	}
	n.W, n.H = p.compute(n, width, wMode, height, hMode, width, height, 0, true)
	n.X, n.Y = s.Margin.Left, s.Margin.Top
}

type pass struct{ id uint32 }

// compute sizes n's border box for the space offered and, when final, lays
// out its children. ownerW/ownerH are the parent's inner size, for
// percentages.
func (p *pass) compute(n *Node, availW float32, wMode Mode, availH float32, hMode Mode, ownerW, ownerH float32, free axes, final bool) (float32, float32) {
	if !final {
		if n.pass != p.id {
			n.pass, n.cache = p.id, n.cache[:0]
		}
		for _, c := range n.cache {
			if c.wMode == wMode && c.hMode == hMode && c.free == free && same(c.availW, availW) && same(c.availH, availH) &&
				same(c.ownerW, ownerW) && same(c.ownerH, ownerH) {
				return c.w, c.h
			}
		}
	}
	w, h := p.layout(n, availW, wMode, availH, hMode, ownerW, ownerH, free, final)
	if !final {
		n.cache = append(n.cache, cacheEntry{availW, availH, ownerW, ownerH, wMode, hMode, free, w, h})
	} else {
		n.W, n.H = w, h
	}
	return w, h
}

func same(a, b float32) bool { return a == b || (!defined(a) && !defined(b)) }

// axis helpers: main/cross views of a node's geometry.
type axis struct{ row bool }

func (a axis) size(w, h float32) float32 {
	if a.row {
		return w
	}
	return h
}

func (a axis) lead(e Edges) float32 {
	if a.row {
		return e.Left
	}
	return e.Top
}

func (a axis) trail(e Edges) float32 {
	if a.row {
		return e.Right
	}
	return e.Bottom
}

func (a axis) both(e Edges) float32 { return a.lead(e) + a.trail(e) }

func (a axis) styleSize(s *Style) (size, min, max Value) {
	if a.row {
		return s.Width, s.MinWidth, s.MaxWidth
	}
	return s.Height, s.MinHeight, s.MaxHeight
}

func (a axis) leadInset(s *Style) Value {
	if a.row {
		return s.Left
	}
	return s.Top
}

func (a axis) trailInset(s *Style) Value {
	if a.row {
		return s.Right
	}
	return s.Bottom
}

func (a axis) setPos(n *Node, v float32) {
	if a.row {
		n.X = v
	} else {
		n.Y = v
	}
}

type item struct {
	n            *Node
	base, hyp    float32 // flex base size, hypothetical main size
	min, max     float32 // main-axis bounds
	target       float32 // final main size
	cross        float32
	frozen       bool
	violation    float32 // this round's clamp adjustment
	marginMain   float32
	marginCross  float32
	stretchCross bool
}

func (p *pass) layout(n *Node, availW float32, wMode Mode, availH float32, hMode Mode, ownerW, ownerH float32, free axes, final bool) (float32, float32) {
	s := &n.Style
	padH := s.Padding.Left + s.Padding.Right + s.Border.Left + s.Border.Right
	padV := s.Padding.Top + s.Padding.Bottom + s.Border.Top + s.Border.Bottom
	minW, maxW := s.MinWidth.resolve(ownerW), s.MaxWidth.resolve(ownerW)
	minH, maxH := s.MinHeight.resolve(ownerH), s.MaxHeight.resolve(ownerH)
	if free&freeW != 0 {
		minW, maxW = undefined, undefined
	}
	if free&freeH != 0 {
		minH, maxH = undefined, undefined
	}
	// The border box can't be smaller than its padding and border.
	minW, minH = max32orDef(minW, padH), max32orDef(minH, padV)

	// The node's own size, where already known.
	w, h := undefined, undefined
	if wMode == Exactly {
		w = availW
	} else if sw := s.Width.resolve(ownerW); defined(sw) {
		w = clamp(sw, minW, maxW)
	}
	if hMode == Exactly {
		h = availH
	} else if sh := s.Height.resolve(ownerH); defined(sh) {
		h = clamp(sh, minH, maxH)
	}
	limitW := limit(w, availW, wMode)
	limitH := limit(h, availH, hMode)

	var flow []*Node
	for _, c := range n.Children {
		if !c.Style.Hidden && c.Style.Position == Relative {
			flow = append(flow, c)
		}
	}

	// Leaves: measured, or sized by padding alone.
	if len(n.Children) == 0 {
		if !defined(w) || !defined(h) {
			cw, ch := float32(0), float32(0)
			if n.Measure != nil {
				iw, im := inner(w, limitW, padH)
				ih, hm := inner(h, limitH, padV)
				cw, ch = n.Measure(iw, im, ih, hm)
			}
			if !defined(w) {
				w = clamp(cw+padH, minW, maxW)
				if defined(limitW) && wMode == AtMost {
					w = min32(w, max32(limitW, minW))
				}
			}
			if !defined(h) {
				h = clamp(ch+padV, minH, maxH)
				if defined(limitH) && hMode == AtMost {
					h = min32(h, max32(limitH, minH))
				}
			}
		}
		if final {
			n.ContentW, n.ContentH = w, h
		}
		return w, h
	}

	ax := axis{row: s.Direction == Row}
	cx := axis{row: !ax.row}
	pbMain, pbCross := ax.size(padH, padV), ax.size(padV, padH)
	size := func(a axis) (float32, float32, Mode) { // own size, limit, mode on an axis
		if a.row {
			return w, limitW, wMode
		}
		return h, limitH, hMode
	}
	mainSize, mainLimit, mainMode := size(ax)
	crossSize, crossLimit, _ := size(cx)

	innerMain := undefined // definite inner main size
	if defined(mainSize) {
		innerMain = mainSize - pbMain
	}
	innerCross := undefined
	if defined(crossSize) {
		innerCross = crossSize - pbCross
	}
	crossAvail := innerCross // a limit for children when not definite
	if !defined(crossAvail) && defined(crossLimit) {
		crossAvail = crossLimit - pbCross
	}
	// Percentages resolve against the definite inner size only.
	pctW, pctH := ax.size(innerMain, innerCross), ax.size(innerCross, innerMain)
	if s.Scroll {
		// Scroll content is unconstrained along the main axis.
		innerMain, mainLimit, mainMode = undefined, undefined, Undefined
		if ax.row {
			pctW = undefined
		} else {
			pctH = undefined
		}
	}

	// 1. Flex base and hypothetical main sizes.
	items := make([]item, len(flow))
	for i, c := range flow {
		cs := &c.Style
		it := &items[i]
		it.n = c
		it.marginMain, it.marginCross = ax.both(cs.Margin), cx.both(cs.Margin)
		sz, mn, mx := ax.styleSize(cs)
		ownerMain := ax.size(pctW, pctH)
		it.min, it.max = mn.resolve(ownerMain), mx.resolve(ownerMain)
		cpad := ax.size(cs.Padding.Left+cs.Padding.Right+cs.Border.Left+cs.Border.Right,
			cs.Padding.Top+cs.Padding.Bottom+cs.Border.Top+cs.Border.Bottom)
		it.min = max32orDef(it.min, cpad)

		align := cs.AlignSelf
		if align == AlignAuto {
			align = s.AlignItems
		}
		cSize, cMin, cMax := cx.styleSize(cs)
		ownerCross := cx.size(pctW, pctH)
		it.stretchCross = (align == AlignAuto || align == AlignStretch) && !defined(cSize.resolve(ownerCross))

		switch b := cs.Basis.resolve(ownerMain); {
		case defined(b):
			it.base = b
		case defined(sz.resolve(ownerMain)):
			it.base = sz.resolve(ownerMain)
		default:
			// Content size: lay the child out with no main limit, and the
			// cross space it will get.
			crossOffer, crossMode := undefined, Undefined
			if defined(crossAvail) {
				crossOffer, crossMode = crossAvail-it.marginCross, AtMost
				if it.stretchCross && defined(innerCross) {
					crossOffer, crossMode = clamp(crossOffer, cMin.resolve(ownerCross), cMax.resolve(ownerCross)), Exactly
				}
			}
			cw, ch := p.offer(c, ax, undefined, Undefined, crossOffer, crossMode, pctW, pctH, true)
			it.base = ax.size(cw, ch)
		}
		it.hyp = clamp(it.base, it.min, it.max)
	}

	gaps := float32(0)
	if len(items) > 1 {
		gaps = s.Gap * float32(len(items)-1)
	}
	sumHyp := gaps
	for _, it := range items {
		sumHyp += it.hyp + it.marginMain
	}

	// 2. The space to flex into.
	avail := innerMain
	if !defined(avail) {
		avail = sumHyp
		if mainMode == AtMost && defined(mainLimit) && sumHyp > mainLimit-pbMain {
			avail = mainLimit - pbMain
		}
		if defined(mainSize) {
			avail = mainSize - pbMain
		}
	}
	resolveFlexible(items, avail-gaps)

	// 3. Cross sizes.
	for i := range items {
		it := &items[i]
		cs := &it.n.Style
		cSize, cMin, cMax := cx.styleSize(cs)
		ownerCross := cx.size(pctW, pctH)
		switch {
		case defined(cSize.resolve(ownerCross)):
			it.cross = clamp(cSize.resolve(ownerCross), cMin.resolve(ownerCross), cMax.resolve(ownerCross))
		case it.stretchCross && defined(innerCross):
			it.cross = clamp(innerCross-it.marginCross, cMin.resolve(ownerCross), cMax.resolve(ownerCross))
		default:
			crossOffer, crossMode := undefined, Undefined
			if defined(crossAvail) {
				crossOffer, crossMode = crossAvail-it.marginCross, AtMost
			}
			cw, ch := p.offer(it.n, ax, it.target, Exactly, crossOffer, crossMode, pctW, pctH, false)
			it.cross = cx.size(cw, ch)
		}
	}
	lineCross := innerCross
	if !defined(lineCross) {
		lineCross = 0
		for _, it := range items {
			lineCross = max32(lineCross, it.cross+it.marginCross)
		}
		_, cMinV, cMaxV := cx.styleSize(s)
		ownerC := ax.size(ownerH, ownerW)
		lineCross = clamp(lineCross+pbCross, max32orDef(cMinV.resolve(ownerC), pbCross), cMaxV.resolve(ownerC)) - pbCross
		if defined(crossLimit) && lineCross > crossLimit-pbCross {
			lineCross = crossLimit - pbCross
		}
		for i := range items {
			if items[i].stretchCross {
				cs := &items[i].n.Style
				_, cMin, cMax := cx.styleSize(cs)
				items[i].cross = clamp(lineCross-items[i].marginCross, cMin.resolve(ax.size(pctH, pctW)), cMax.resolve(ax.size(pctH, pctW)))
			}
		}
	}

	// The node's own size.
	if !defined(mainSize) {
		_, mn, mx := ax.styleSize(s)
		ownerMain := ax.size(ownerW, ownerH)
		mainSize = clamp(avail+pbMain, max32orDef(mn.resolve(ownerMain), pbMain), mx.resolve(ownerMain))
	}
	if !defined(crossSize) {
		crossSize = lineCross + pbCross
	}
	if ax.row {
		w, h = mainSize, crossSize
	} else {
		w, h = crossSize, mainSize
	}
	if !final {
		return w, h
	}

	// 4. Final layout of each child, then position it.
	used := gaps
	for _, it := range items {
		used += it.target + it.marginMain
	}
	spare := mainSize - pbMain - used
	if s.Scroll {
		spare = 0
	}
	lead, between := justify(s.Justify, spare, len(items))
	pos := ax.lead(s.Padding) + ax.lead(s.Border) + lead
	contentMain, contentCross := float32(0), float32(0)
	for _, it := range items {
		c := it.n
		cs := &c.Style
		if ax.row {
			p.compute(c, it.target, Exactly, it.cross, Exactly, pctW, pctH, 0, true)
		} else {
			p.compute(c, it.cross, Exactly, it.target, Exactly, pctW, pctH, 0, true)
		}
		pos += ax.lead(cs.Margin)
		ax.setPos(c, pos+relOffset(ax, cs, pctW, pctH))
		pos += it.target + ax.trail(cs.Margin) + s.Gap + between

		align := cs.AlignSelf
		if align == AlignAuto {
			align = s.AlignItems
		}
		room := lineCross - it.cross - it.marginCross
		off := float32(0)
		switch align {
		case AlignCenter:
			off = room / 2
		case AlignEnd:
			off = room
		}
		cpos := cx.lead(s.Padding) + cx.lead(s.Border) + cx.lead(cs.Margin) + off
		cx.setPos(c, cpos+relOffset(cx, cs, pctW, pctH))
		contentCross = max32(contentCross, cpos+it.cross+cx.trail(cs.Margin))
	}
	contentMain = pos - s.Gap - between + ax.trail(s.Padding)
	if len(items) == 0 {
		contentMain = ax.lead(s.Padding) + ax.lead(s.Border) + ax.trail(s.Padding)
	}
	contentCross += cx.trail(s.Padding)
	if ax.row {
		n.ContentW, n.ContentH = max32(contentMain, w-ax.trail(s.Border)), max32(contentCross, h-cx.trail(s.Border))
	} else {
		n.ContentW, n.ContentH = max32(contentCross, w-cx.trail(s.Border)), max32(contentMain, h-ax.trail(s.Border))
	}

	// 5. Absolutely positioned children, against the padding box.
	for _, c := range n.Children {
		if !c.Style.Hidden && c.Style.Position == Absolute {
			p.absolute(n, c, w, h, ax)
		}
	}
	for _, c := range n.Children {
		if c.Style.Hidden {
			c.X, c.Y, c.W, c.H = 0, 0, 0, 0
		}
	}
	return w, h
}

// offer lays c out with a main-axis and cross-axis offer and returns its
// border-box size.
// With freeMain, the child ignores its own main-axis min/max (flex base size).
func (p *pass) offer(c *Node, ax axis, main float32, mainMode Mode, cross float32, crossMode Mode, pctW, pctH float32, freeMain bool) (float32, float32) {
	var free axes
	if freeMain {
		free = freeH
		if ax.row {
			free = freeW
		}
	}
	if ax.row {
		return p.compute(c, main, mainMode, cross, crossMode, pctW, pctH, free, false)
	}
	return p.compute(c, cross, crossMode, main, mainMode, pctW, pctH, free, false)
}

// resolveFlexible grows or shrinks items to fill space (CSS Flexbox §9.7).
func resolveFlexible(items []item, space float32) {
	sumHyp := float32(0)
	for _, it := range items {
		sumHyp += it.hyp + it.marginMain
	}
	growing := sumHyp < space
	for i := range items {
		it := &items[i]
		it.target = it.hyp
		factor := it.n.Style.Shrink
		if growing {
			factor = it.n.Style.Grow
		}
		it.frozen = factor == 0 || (growing && it.base > it.hyp) || (!growing && it.base < it.hyp) || !defined(space)
	}
	for {
		var free, sumFactor, sumScaled float32
		free = space
		unfrozen := 0
		for _, it := range items {
			free -= it.marginMain
			if it.frozen {
				free -= it.target
			} else {
				free -= it.base
				unfrozen++
				sumFactor += factorOf(it, growing)
				sumScaled += it.n.Style.Shrink * it.base
			}
		}
		if unfrozen == 0 {
			return
		}
		if sumFactor < 1 {
			// Flex factors that add up to less than 1 take only that share.
			initial := free
			free = free * sumFactor
			if abs32(initial) < abs32(free) {
				free = initial
			}
		}
		// Distribute, clamp, and freeze the items whose clamping goes the
		// same way as the total (min violations when it grew, max when it
		// shrank); repeat until every item is frozen.
		var violation float32
		for i := range items {
			it := &items[i]
			if it.frozen {
				continue
			}
			t := it.base
			switch {
			case growing && sumFactor > 0:
				t += free * it.n.Style.Grow / sumFactor
			case !growing && sumScaled > 0:
				t += free * (it.n.Style.Shrink * it.base) / sumScaled
			}
			c := clamp(t, it.min, it.max)
			if c < 0 {
				c = 0
			}
			it.violation = c - t
			violation += c - t
			it.target = c
		}
		for i := range items {
			it := &items[i]
			if it.frozen {
				continue
			}
			if v := it.violation; violation == 0 || (violation > 0 && v > 0) || (violation < 0 && v < 0) {
				it.frozen = true
			}
		}
	}
}

func factorOf(it item, growing bool) float32 {
	if growing {
		return it.n.Style.Grow
	}
	return it.n.Style.Shrink
}

// justify returns the offset before the first child and the extra space
// between children.
func justify(j Justify, free float32, count int) (lead, between float32) {
	if count == 0 {
		return 0, 0
	}
	switch j {
	case JustifyCenter:
		return free / 2, 0
	case JustifyEnd:
		return free, 0
	case JustifySpaceBetween:
		if free < 0 || count == 1 {
			return 0, 0
		}
		return 0, free / float32(count-1)
	case JustifySpaceAround:
		if free < 0 {
			return 0, 0 // overflowing: start, as browsers do
		}
		return free / float32(count) / 2, free / float32(count)
	case JustifySpaceEvenly:
		if free < 0 {
			return 0, 0
		}
		return free / float32(count+1), free / float32(count+1)
	}
	return 0, 0
}

// relOffset is a relatively positioned child's inset shift on an axis.
func relOffset(a axis, s *Style, pctW, pctH float32) float32 {
	owner := a.size(pctW, pctH)
	if l := a.leadInset(s).resolve(owner); defined(l) {
		return l
	}
	if t := a.trailInset(s).resolve(owner); defined(t) {
		return -t
	}
	return 0
}

// absolute sizes and places an absolutely positioned child within the
// padding box of n (w × h is n's border box).
func (p *pass) absolute(n, c *Node, w, h float32, ax axis) {
	s, cs := &n.Style, &c.Style
	boxW := w - s.Border.Left - s.Border.Right
	boxH := h - s.Border.Top - s.Border.Bottom
	left, right := cs.Left.resolve(boxW), cs.Right.resolve(boxW)
	top, bottom := cs.Top.resolve(boxH), cs.Bottom.resolve(boxH)
	mH, mV := cs.Margin.Left+cs.Margin.Right, cs.Margin.Top+cs.Margin.Bottom

	cw, wMode := cs.Width.resolve(boxW), Exactly
	if !defined(cw) {
		if defined(left) && defined(right) {
			cw = boxW - left - right - mH
		} else {
			cw, wMode = boxW-mH-zeroIfUndef(left)-zeroIfUndef(right), AtMost
		}
	} else {
		cw = clamp(cw, cs.MinWidth.resolve(boxW), cs.MaxWidth.resolve(boxW))
	}
	ch, hMode := cs.Height.resolve(boxH), Exactly
	if !defined(ch) {
		if defined(top) && defined(bottom) {
			ch = boxH - top - bottom - mV
		} else {
			ch, hMode = undefined, Undefined
		}
	} else {
		ch = clamp(ch, cs.MinHeight.resolve(boxH), cs.MaxHeight.resolve(boxH))
	}
	if wMode != Exactly || hMode != Exactly {
		cw, ch = p.compute(c, cw, wMode, ch, hMode, boxW, boxH, 0, false)
	}
	p.compute(c, cw, Exactly, ch, Exactly, boxW, boxH, 0, true)

	// Unset insets: the static position, as if c were the only flex item.
	place := func(a axis, lead, trail, size, box, margin, leadM, border float32) float32 {
		switch {
		case defined(lead):
			return border + lead + leadM
		case defined(trail):
			return border + box - trail - size - (margin - leadM)
		}
		pad := a.lead(s.Padding)
		innerBox := box - a.both(s.Padding)
		room := innerBox - size - margin
		var off float32
		if a.row == ax.row {
			off, _ = justify(s.Justify, room, 1)
		} else {
			align := cs.AlignSelf
			if align == AlignAuto {
				align = s.AlignItems
			}
			switch align {
			case AlignCenter:
				off = room / 2
			case AlignEnd:
				off = room
			}
		}
		return border + pad + leadM + off
	}
	c.X = place(axis{row: true}, left, right, cw, boxW, mH, cs.Margin.Left, s.Border.Left)
	c.Y = place(axis{row: false}, top, bottom, ch, boxH, mV, cs.Margin.Top, s.Border.Top)
}

// limit is the most space a node may take on an axis: its own size if
// known, else the offer when it is a limit.
func limit(own, avail float32, m Mode) float32 {
	if defined(own) {
		return own
	}
	if m != Undefined {
		return avail
	}
	return undefined
}

// inner converts a border-box size or limit into the content-box offer for
// a measure function.
func inner(own, lim, pad float32) (float32, Mode) {
	if defined(own) {
		return max32(own-pad, 0), Exactly
	}
	if defined(lim) {
		return max32(lim-pad, 0), AtMost
	}
	return undefined, Undefined
}

func max32orDef(v, floor float32) float32 {
	if !defined(v) || v < floor {
		return floor
	}
	return v
}

func zeroIfUndef(v float32) float32 {
	if defined(v) {
		return v
	}
	return 0
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
