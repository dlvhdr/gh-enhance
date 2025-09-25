package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/v2/spinner"
)

const (
	headerHeight    = 4
	footerHeight    = 1
	smallScreen     = 130
	paneTitleHeight = 1

	unfocusedLargePaneWidth = 20
	focusedLargePaneWidth   = 40

	focusedSmallPaneWidth = 25
)

const (
	CanceledIcon = ""
	SkippedIcon  = ""
	WaitingIcon  = ""
	PendingIcon  = ""
	FailureIcon  = "󰅙"
	SuccessIcon  = ""
	MergedIcon   = ""
	DraftIcon    = ""
	OpenIcon     = ""
	ClosedIcon   = ""

	AsciiSkippedIcon = `
    ,---_   
   /   ╱ \  
  (   ╱   ) 
   \ ╱   /  
		:---:   
		`

	AsciiStoppedIcon = `
    .---.   
   /  │  \  
  |   │   | 
   \  .  /  
		:---:
		`

	Separator    = "│"
	ExpandSymbol = "▶"
	ListSymbol   = "≡"
	Ellipsis     = "…"
	ParentLogo   = "〓"

	Logo = `▐▔▔▐▚ ▌▐ ▌▐▔▌▐▚ ▌▐▔▔▐▔▔
▐▛▁▐ ▚▌▐▔▌▐▔▌▐ ▚▌▐▁▁▐▛▁`
	// ⟋⟋ ⧸⧸ ╱╱ ⚌ ⩵ ⫽ 〓⤬ ⊜
)

var LogsFrames = spinner.Spinner{
	Frames: []string{
		`
 ╭─────────╮   
 │FEDGHHFUR│   
 │ORUDKFMVR│   
 │NFEYNFDSN│   
 │NFEYFNWYF│   
 │MADODJWJF│   
 │FHUEHFISI│──╮
 │YFURKSIFJ│╭╮│
 │UDYGJDIUW│─╯│
 ╰─────────╰──╯
               
  Loading.     
`,
		`
 ╭─────────╮   
 │-1101100-│   
 │-1101101-│   
 │-1100001-│   
 │-1101111-│   
 │-1101100-│   
 │-1101111-│──╮
 │-1101100-│╭╮│
 │-1100101-│─╯│
 ╰─────────╰──╯
               
  Loading.     
`,
		`
 ╭─────────╮   
 │^№№*%^)?:│   
 │)(&№:?@!~│   
 │/\'"[]{&$│   
 │$^()%&^$#│   
 │#$%"%;&^&│   
 │^&^??\/\"│──╮
 │^%%*&#()$│╭╮│
 │(?*;?%%^&│─╯│
 ╰─────────╰──╯
               
  Loading.     
`,
		`
 ╭─────────╮   
 │654130037│   
 │103985647│   
 │376247259│   
 │184537563│   
 │184764464│   
 │104749275│──╮
 │367858324│╭╮│
 │438756456│─╯│
 ╰─────────╰──╯
               
  Loading.     
`,
		`
 ╭─────────╮   
 │-1101100-│   
 │-1101101-│   
 │-1100001-│   
 │-1101111-│   
 │-1101100-│   
 │-1101111-│──╮
 │-1101100-│╭╮│
 │-1100101-│─╯│
 ╰─────────╰──╯
               
  Loading..    
`,
		`
 ╭─────────╮   
 │FEDGHHFUR│   
 │ORUDKFMVR│   
 │NFEYNFDSN│   
 │NFEYFNWYF│   
 │MADODJWJF│   
 │FHUEHFISI│──╮
 │YFURKSIFJ│╭╮│
 │UDYGJDIUW│─╯│
 ╰─────────╰──╯
               
  Loading..    
`,
		`
 ╭─────────╮   
 │-+-+-+-+-│   
 │-+-+-+-+-│   
 │-+-+-+-+-│   
 │-+-+-+-+-│   
 │-+-+-+-+-│   
 │-+-+-+-+-│──╮
 │-+-+-+-+-│╭╮│
 │-+-+-+-+-│─╯│
 ╰─────────╰──╯
               
  Loading...   
`,
		`
 ╭─────────╮   
 │^№№*%^)?:│   
 │)(&№:?@!~│   
 │/\'"[]{&$│   
 │$^()%&^$#│   
 │#$%"%;&^&│   
 │^&^??\\/"│──╮
 │^%%*&#()$│╭╮│
 │(?*;?%%^&│─╯│
 ╰─────────╰──╯
               
  Loading...   
`,
		`
 ╭─────────╮   
 │654130037│   
 │103985647│   
 │376247259│   
 │184537563│   
 │184764464│   
 │104749275│──╮
 │367858324│╭╮│
 │438756456│─╯│
 ╰─────────╰──╯
               
  Loading...   
`,
	},
	FPS: time.Second / 10,
}

var InProgressFrames = spinner.Spinner{
	Frames: []string{
		`
  ▀▀ 
     
     
`,

		`
   ▀▜
     
     
`,
		`
    ▜
    ▐
     
`,
		`
     
    ▐
    ▟
`,
		`
     
     
   ▄▟
`,
		`
     
     
  ▄▄ 
`,
		`
     
     
 ▄▄  
`,
		`
     
     
▙▄   
`,
		`
     
▌    
▙    
`,
		`
▛    
▌    
     
`,
		`
▛▀   
     
     
`,
		`
 ▀▀  
     
     
`,
	},
	FPS: time.Second / 12,
}

var ClockFrames = spinner.Spinner{
	Frames: []string{"󱑌", "󱑍", "󱑎", "󱑏", "󱑐", "󱑑", "󱑒", "󱑓", "󱑔", "󱑕", "󱑖", "󱑋"},
	FPS:    time.Second / 6,
}
