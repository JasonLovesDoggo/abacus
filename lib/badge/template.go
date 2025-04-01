package badge

// Templates for different badge styles
const (
	// templateFlatStyle is the SVG template for flat style badges
	templateFlatStyle = `
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{{.TotalWidth}}" height="{{.Height}}">
  <linearGradient id="smooth" x2="0" y2="100%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <mask id="round">
    <rect width="{{.TotalWidth}}" height="{{.Height}}" rx="{{calcRadius .Height}}" fill="#fff"/>
  </mask>
  <g mask="url(#round)">
    <rect width="{{.LeftWidth}}" height="{{.Height}}" fill="#555"/>
    <rect x="{{.LeftWidth}}" width="{{.RightWidth}}" height="{{.Height}}" fill="{{.Color}}"/>
    <rect width="{{.TotalWidth}}" height="{{.Height}}" fill="url(#smooth)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="{{.FontFamily}}" font-size="{{.FontSize}}">
    {{if ne .LeftText ""}}
      <text x="{{.LeftTextX}}" y="{{.TextY}}" fill="#010101" fill-opacity=".3">{{.LeftText}}</text>
      <text x="{{.LeftTextX}}" y="{{.TextY}}" fill="#fff">{{.LeftText}}</text>
    {{end}}
    <text x="{{.RightTextX}}" y="{{.TextY}}" fill="#010101" fill-opacity=".3">{{.RightText}}</text>
    <text x="{{.RightTextX}}" y="{{.TextY}}" fill="#fff">{{.RightText}}</text>
  </g>
</svg>
`

	// templateFlatSquareStyle is the SVG template for flat-square style badges
	templateFlatSquareStyle = `
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{{.TotalWidth}}" height="{{.Height}}">
  <g>
    <rect width="{{.LeftWidth}}" height="{{.Height}}" fill="#555"/>
    <rect x="{{.LeftWidth}}" width="{{.RightWidth}}" height="{{.Height}}" fill="{{.Color}}"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="{{.FontFamily}}" font-size="{{.FontSize}}">
    {{if ne .LeftText ""}}
      <text x="{{.LeftTextX}}" y="{{.TextY}}">{{.LeftText}}</text>
    {{end}}
    <text x="{{.RightTextX}}" y="{{.TextY}}">{{.RightText}}</text>
  </g>
</svg>
`

	// templatePlasticStyle is the SVG template for plastic style badges
	templatePlasticStyle = `
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{{.TotalWidth}}" height="{{.Height}}">
  <linearGradient id="gradient" x2="0" y2="100%">
    <stop offset="0%" stop-color="#fff" stop-opacity=".7"/>
    <stop offset="100%" stop-opacity=".1"/>
  </linearGradient>
  <mask id="round">
    <rect width="{{.TotalWidth}}" height="{{.Height}}" rx="{{calcRadius .Height}}" fill="#fff"/>
  </mask>
  <g mask="url(#round)">
    <rect width="{{.LeftWidth}}" height="{{.Height}}" fill="#555"/>
    <rect x="{{.LeftWidth}}" width="{{.RightWidth}}" height="{{.Height}}" fill="{{.Color}}"/>
    <rect width="{{.TotalWidth}}" height="{{.Height}}" fill="url(#gradient)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="{{.FontFamily}}" font-size="{{.FontSize}}">
    {{if ne .LeftText ""}}
      <text x="{{.LeftTextX}}" y="{{.TextY}}" fill="#010101" fill-opacity=".3">{{.LeftText}}</text>
      <text x="{{.LeftTextX}}" y="{{.TextY}}">{{.LeftText}}</text>
    {{end}}
    <text x="{{.RightTextX}}" y="{{.TextY}}" fill="#010101" fill-opacity=".3">{{.RightText}}</text>
    <text x="{{.RightTextX}}" y="{{.TextY}}">{{.RightText}}</text>
  </g>
</svg>
`

	// templateFlatSimpleStyle is the SVG template for flat-simple style badges
	templateFlatSimpleStyle = `
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{{.RightWidth}}" height="{{.Height}}">
  <linearGradient id="smooth" x2="0" y2="100%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <mask id="round">
    <rect width="{{.RightWidth}}" height="{{.Height}}" rx="{{calcRadius .Height}}" fill="#fff"/>
  </mask>
  <g mask="url(#round)">
    <rect width="{{.RightWidth}}" height="{{.Height}}" fill="{{.Color}}"/>
    <rect width="{{.RightWidth}}" height="{{.Height}}" fill="url(#smooth)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="{{.FontFamily}}" font-size="{{.FontSize}}">
    <text x="{{.RightWidth | div 2}}" y="{{.TextY}}" fill="#010101" fill-opacity=".3">{{.RightText}}</text>
    <text x="{{.RightWidth | div 2}}" y="{{.TextY}}" fill="#fff">{{.RightText}}</text>
  </g>
</svg>
`

	// templateFlatSquareSimpleStyle is the SVG template for flat-square-simple style badges
	templateFlatSquareSimpleStyle = `
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{{.RightWidth}}" height="{{.Height}}">
  <g>
    <rect width="{{.RightWidth}}" height="{{.Height}}" fill="{{.Color}}"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="{{.FontFamily}}" font-size="{{.FontSize}}">
    <text x="{{.RightWidth | div 2}}" y="{{.TextY}}">{{.RightText}}</text>
  </g>
</svg>
`

	// templatePlasticSimpleStyle is the SVG template for plastic-simple style badges
	templatePlasticSimpleStyle = `
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{{.RightWidth}}" height="{{.Height}}">
  <linearGradient id="gradient" x2="0" y2="100%">
    <stop offset="0%" stop-color="#fff" stop-opacity=".7"/>
    <stop offset="100%" stop-opacity=".1"/>
  </linearGradient>
  <mask id="round">
    <rect width="{{.RightWidth}}" height="{{.Height}}" rx="{{calcRadius .Height}}" fill="#fff"/>
  </mask>
  <g mask="url(#round)">
    <rect width="{{.RightWidth}}" height="{{.Height}}" fill="{{.Color}}"/>
    <rect width="{{.RightWidth}}" height="{{.Height}}" fill="url(#gradient)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="{{.FontFamily}}" font-size="{{.FontSize}}">
    <text x="{{.RightWidth | div 2}}" y="{{.TextY}}" fill="#010101" fill-opacity=".3">{{.RightText}}</text>
    <text x="{{.RightWidth | div 2}}" y="{{.TextY}}">{{.RightText}}</text>
  </g>
</svg>
`
)
