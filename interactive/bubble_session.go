package interactive

import (
	"context"
	"errors"
	"fmt"
	"io" // Added for io.EOF check
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	// "golang.org/x/term" // Removed for WASM compatibility

	// Use local ui paths
	"github.com/tmc/cgpt/ui/debug"
	"github.com/tmc/cgpt/ui/editor"  // Use local editor package
	"github.com/tmc/cgpt/ui/help"    // Use local help package
	"github.com/tmc/cgpt/ui/history" // Corrected history import path
	"github.com/tmc/cgpt/ui/keymap"
	"github.com/tmc/cgpt/ui/message"
	"github.com/tmc/cgpt/ui/spinner"
	"github.com/tmc/cgpt/ui/statusbar"
	"github.com/tmc/cgpt/ui/textinput" // For command mode input
)

// Helper function to create a command that sends a message
func msgCmd(msg tea.Msg) tea.Cmd {
	return func() tea.Msg { return msg }
}

// --- Compile-time check for Session interface ---
var _ Session = (*BubbleSession)(nil)

// --- Editor Mode Enum ---
type editorMode int

const (
	modeInsert  editorMode = iota // Normal text editing
	modeCommand                   // Command mode (after Escape)
)

// --- BubbleSession Implementation ---

// BubbleSession implements the Session interface using Bubble Tea.
type BubbleSession struct {
	config  Config
	model   *bubbleModel
	program *tea.Program
}

// NewBubbleSession creates a new Bubble Tea based session.
func NewBubbleSession(cfg Config) (*BubbleSession, error) {
	if cfg.SingleLineHint == "" {
		cfg.SingleLineHint = DefaultSingleLineHint
	}
	if cfg.MultiLineHint == "" {
		cfg.MultiLineHint = DefaultMultiLineHint
	}

	session := &BubbleSession{config: cfg}
	return session, nil
}

// Run starts the Bubble Tea application loop.
func (s *BubbleSession) Run(ctx context.Context) error {
	// Initialize components that will be part of sub-models
	editorModel := editor.New()
	editorModel.SetHistory(s.config.LoadedHistory)
	editorModel.Focus()

	cmdInput := textinput.New()
	cmdInput.Prompt = ":"
	cmdInput.CharLimit = 200
	cmdInput.Width = 50 // Initial width

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	helpModel := help.New(keymap.DefaultKeyMap())

	// Initialize the main model with embedded sub-models
	s.model = &bubbleModel{
		session: s,
		ctx:     ctx,
		input: InputModel{
			editor:       editorModel,
			commandInput: cmdInput,
			mode:         modeInsert, // Start in insert mode
			keyMap:       keymap.DefaultKeyMap(),
			lastInput:    s.config.LastInput,
		},
		conversationVM: ConversationViewModel{
			conversation: []message.Msg{}, // Initialize empty conversation
			respBuffer:   strings.Builder{},
		},
		status: StatusModel{
			spinner: sp,
			help:    helpModel,
			// Other fields like currentErr, isProcessing etc. default to zero values
		},
		debug:    debug.NewView(),
		handlers: createEventHandlers(),
		// quitting defaults to false
	}

	var options []tea.ProgramOption
	options = append(options, tea.WithAltScreen(), tea.WithMouseAllMotion())
	if s.config.Stdin != nil {
		if f, ok := s.config.Stdin.(*os.File); ok {
			options = append(options, tea.WithInput(f))
		}
	}

	s.program = tea.NewProgram(s.model, options...)

	progDone := make(chan error, 1)
	go func() { _, runErr := s.program.Run(); progDone <- runErr }()

	select {
	case <-ctx.Done():
		s.model.debug.Log(" > Context cancelled, quitting program...")
		s.program.Quit()
		<-progDone
		return ctx.Err()
	case err := <-progDone:
		if ctx.Err() != nil {
			s.model.debug.Log(" > Program finished after context cancellation.")
			return ctx.Err()
		}
		s.model.debug.Log(" > Program finished.")
		return err
	}
}

// SetStreaming updates the UI state for streaming via a message.
func (s *BubbleSession) SetStreaming(streaming bool) {
	if s.program != nil {
		s.program.Send(streamingMsg(streaming))
	}
}

// SetLastInput updates the last input for history recall in the editor model.
func (s *BubbleSession) SetLastInput(input string) {
	if s.model != nil {
		s.model.input.editor.SetLastInput(input) // Update through input model
	}
	s.config.LastInput = input
}

// AddResponsePart sends a part of the response to the Bubble Tea model via a message.
func (s *BubbleSession) AddResponsePart(part string) {
	if s.program != nil {
		s.program.Send(addResponsePartMsg{part: part})
	}
}

// GetHistory retrieves the final history from the editor model.
func (s *BubbleSession) GetHistory() []string {
	if s.model != nil {
		return s.model.input.editor.GetHistory() // Get from input model
	}
	return s.config.LoadedHistory // Fallback
}

// GetHistoryFilename retrieves the configured history filename.
func (s *BubbleSession) GetHistoryFilename() string {
	return s.config.HistoryFile // Return path from config
}

// LoadHistory delegates history loading to the editor model.
func (s *BubbleSession) LoadHistory(filename string) error {
	if s.model == nil {
		return errors.New("session model not initialized")
	}
	h, err := history.Load(filename)
	if err != nil {
		s.model.debug.Log(" > Failed to load history: %v", err)
		return nil // Non-fatal error
	}
	s.model.input.editor.SetHistory(h) // Update through input model
	s.config.HistoryFile = filename
	s.model.debug.Log(" > History loaded from %s", filename)
	return nil
}

// SaveHistory delegates history saving to the history package.
func (s *BubbleSession) SaveHistory(filename string) error {
	if s.model == nil {
		return errors.New("session model not initialized")
	}
	h := s.model.input.editor.GetHistory() // Get from input model
	if err := history.Save(h, filename); err != nil {
		s.model.debug.Log(" > Failed to save history: %v", err)
		return nil // Non-fatal error
	}
	s.config.HistoryFile = filename
	s.model.debug.Log(" > History saved to %s", filename)
	return nil
}

// Quit signals the Bubble Tea program to quit.
func (s *BubbleSession) Quit() {
	if s.program != nil {
		s.program.Quit()
	}
}

// --- Sub-Models ---

type InputModel struct {
	editor       editor.Model    // Main multi-line editor
	commandInput textinput.Model // Single-line input for command mode
	mode         editorMode      // Current mode (Insert or Command)
	keyMap       keymap.KeyMap
	lastInput    string
	conTODOeAuMrur  ilrle tate for the Bubble Tea UI, composing sub-models.
session       *BubbleSession // Reference back to session for config/callbacks

status         StatusModel

	// Other state
	debug    AguwIn,,ho *bbleModel) (tea.Model, tea.Cmd)
o//o)ODO   (mdbo bt{dng  
	commandResultMsg struct {
		output string
		err    error
	} // Result of executing a command mode command
	// Message history updates
	addUserMessageMsg   struct{ content string }
	addModelMessageMsg  struct{ content string }
	addSystemMessageMsg struct{ content string }
)

funcTODOAcdasamose moIn
}

			m.debug.Log(" > Processing finished")
			if m.status.isStreaming {
				m.status.isStreaming = false
			}
			if m.conversationVM.respBuffer.Len() > 0 {
				cmds = append(cmds, msgCmd(addModelMessageMsg{content: m.conversationVM.respBuffer.String()}))
			}
			m.input.editor.Focus() // Update input model
		}
	case streamingMsg:
		m.status.isStreaming = bool(msg) // Update status model
		m.debug.Log(" > Streaming state: %v", m.status.isStreaming)
		if m.status.isStreaming {
			m.status.currentErr = nil
			cmds = append(cmds, m.status.spinner.Tick)
		} else {
			m.debug.Log(" > Streaming finished")
			if m.conversationVM.respBuffer.Len() > 0 {
				cmds = append(cmds, msgCmd(addModelMessageMsg{content: m.conversationVM.respBuffer.String()}))
			}
			if !m.status.isProcessing {
				m.input.editor.Focus() // Update input model
			}
		}
	case addResponsePartMsg:
		m.conversationVM.respBuffer.WriteString(msg.part) // Update conversationVM
		m.debug.Log(" > Received response part (%d bytes)", len(msg.part))
		if m.status.isProcessing || m.status.isStreaming {
			cmds = append(cmds, m.status.spinner.Tick)
		}

	case submitBufferMsg:
		trimmedInput := strings.TrimSpace(msg.input)
		if trimmedInput != "" && !m.status.isProcessing { // Check status model
			cmds = append(cmds, msgCmd(addUserMessageMsg{content: trimmedInput}))
			m.status.isProcessing = true // Update status model
			m.status.currentErr = nil
			m.debug.Log(" > Submitting input (from msg): '%s'", trimmedInput)
			cmds = append(cmds, m.status.spinner.Tick, m.triggerProcessFn(trimmedInput))
		}

	case editorFinishedMsg:
		m.debug.Log(" > Editor finished msg received")
		if msg.err != nil {
			m.debug.Log(" > Editor error: %v", msg.err)
			m.status.currentErr = msg.err // Update status model
		} else {
			m.debug.Log(" > Applying editor content")
			m.input.editor.SetValue(msg.content) // Update input model
		}
		cmds = append(cmds, m.input.editor.Focus())

	case commandResultMsg:
		m.debug.Log(" > Command result received")
		if msg.err != nil {
			m.status.currentErr = msg.err // Update status model
		} else if msg.output != "" {
			cmds = append(cmds, msgCmd(addSystemMessageMsg{content: msg.output}))
		}
		m.input.mode = modeInsert // Update input model
		cmds = append(cmds, m.input.editor.Focus())
 --- Message History Updat // Start spinner tickes ---
	case addUserMessageMsg:
		m.conversationVM.conversation = append(m.conversationVM.conversation, message.Msg{Type: message.MsgTypeUser, Content: msg.content, Time: time.Now()}) // Update conversationVM
		m.debug.Log(" > Added user message")
	case addModelMessageMsg:
		m.conversationVM.conversation = append(m.conversationVM.conversation, message.Msg{Type: message.MsgTypeAssistant, Content: msg.content, Time: time.Now()}) // Update conversationVM
		m.conversationVM.respBuffer.Reset()
		m.debug.Log(" > Added model message")
	case addSystemMessageMsg:
		m.conversationVM.conversation = append(m.conversationVM.conversation, message.Msg{Type: message.MsgTypeSystem, Content: msg.content, Time: time.Now()}) // Update conversationVM
	(
	// --- Spinner ---
	caseUseer.TMap from ckputmod
		if m.status.isProcessing || m.status.isStreaming { // Check status model
			m.status.spinner, cmd
		}

	case tes.WitaowSizuMsg:
		re.usnnm.han.lerU.onWindowSize(m,pmsg)
	aase errM(g:
		m.debug.Log("s> Err)r Received: %v", /sg.err)
		m.stat/s.currenUErr = msg.err // Uddatte tatss tus mdel
		m.status.isProcessing = falsecmds = append(cmds, cmd)
		m.status.isStreaming= alse
		ifm.iput.mde == modeInser{
			m.pu.dito.Fous()
		} lse {
			m.inu.commanInput.Focus()
		}
		return mni
	}

	//-- Me-SpcificHing---
	swchm.input.mod {	}
se modeInrt:
		cmd =m.updaIsertMo(m)
	/cmds =TapDend(cmds, cmO)
	c:sDemodeComlead:
		cmd =tm.updateComeandM up(msg)
		cmdsa=toppe d(cmss,ucmb)
	}

	// --- Shar-d Mossade Hendling ---
	swipchompg := ysg.(typ){
	cseprocssngMsg:
	//.status isProcess.ngut,bool( cm) // Up ate stamus modelinput.Update(msg)
	//.status currmn=Err a nil
		ifpp.statuedisProccssmnd { cmd)
		// m.cog.Log(" > Processinn started")
			m.conversationVM.respBuffervReset() // ersatM co,ver atcmdVM
			cmd  = append=cmds,  .status.mpinnercTncksationVM.Update(msg)
	/}cdsse {
			m.debug.Log(" > Proc ssin= finished")
			if m.st tus.isSarpamingp{
				m.status.ieStreamnng = fals(
			}
			ifcm.monversstionVM.respBuffer.Le,() > 0 {
				cmds = appcnd(cmdm,dmsgCmd)addModMessageMsg{content:m.covratonVM.respBuffer.Sring()})
			}
		//.input.editor Focum() // Updste inpats ocel
		}
	case streamingMsg:
		= m.statuisSdraaming = boolte(msg) Updates m
		m.debug.Log(" >Stremig state: %v", m.status.isStraming)
		if m.status.iStreaming{
			m.status.currntErr = ni
			cmds = apend(cmds,m.stats.sinner.Tick)
		} else {
			m.debug.Log(" > Streaming finishe")
			if m.conversionVM.respBuffer.Ln() > 0 {
			// cmds = append(cmdsmsgCmd(addModelMessageMsg{,ontent:  .conversationVM.respBuffer.String()})m
			}
			if !m.status.isProcessing {d)
		m.input.editor.Focus()  Updatei m
			}
		}
	casdRsponeParMsg:
		m.converatiVM.repBuffer.WrteStr(sg.prt) et Updateucn m, tea.Batc
		m.debug.Log(" >hRececved responsmsp.rt (%) byt)",len(msg.part))
		f m.satu.isPrcessig|| m.tatus.sStream{
			cms = ppend(cds, m.status.spnner.Tik)
}}

	cas submiBffeMsg:
		trimmedIput:=sringsTrimSpeginput)
		if trimmedInput != "" && !mstatusisProcessing{ Chck satsmod
			cmds= ppend(cmd,mgCmd(addUsrMessageMsg{content:trmmedInput}))
			m.ttus.isProcessig = true // Upat status moel
/		m.st tuu.currpntErrd= nil
			m.tebug.Log(" > SubmeItrtg Mnput (from mon)dl'%s'", trimmpdInpae)
			cmdss= append(ceds,nm.status.spinn r.Tick, m.nrigg nProcessFo(trimmedInput))
		}

	crseal editFinishedMsg:ode.
func (m *bubbleModel) updateInsertMode(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	var cmd tea.Cmd
 Use keyic rrntE = M//gU :ate gtatus(eoaelKeyMsg); ok && keyMsg.Type == tea.KeyEscape {
		m.input.mode = modeCommand // Update input model
		m.input.editor.Blur()
		m.input.commandInput.Reset() m.dUp a eic Com mand Mode")
		}
		return textinput.Blink

	// Delegate to editor 
	m.input.editor, cmd = m.input.editor.Update(msg)
	cmds = append(cmds, cmd)
 m.inputncsrrentE & = input.et//eUItate mtatusipouel.editor.Value()
		m.input.lastInput = submittedInput // Update input model
		ccm=pc=gdppirp(smdmedtlgCmtor.Err, io.EOF) || errors.Is(m.input.editor.Err, editor.ErrInputAborted) {
			m.quitting = true
		p else { // Update input model
			m.status.currentErr = m.input.editor.Err // Update status model

	//---Msag HisoyUpdtes ---
addUesaeMs:
		m.convsainVM.nvrsa=ppe(m.ve satUo VM.converdae ow, metsagehMsg{iyas: oedsaeelMsgTyeUser,tConahpc:mu.g.conhenl, Tpmm: t)me.Now()})Updat onverinVM
cmm.debug.Log(" > =d eappend(cmds, e")
	cascmaddModelMessgeMsg:
		m.cappend(conversion, mssge.Mg{Typ: m.sgTypeAssistant, Content: m., Time time.Now()}) //Updae convesatonVM
		.convrsatioVM.resBffer.Rese(
reu.rebug.Log(" >eAddac comel..essage"
caseaddysemMaeMg:
converaionVM.conversionappend(converaionVM.conversion, mesageMsg{Ty: mae.TypeSysp,CContant: d g.contenth(m*mb:leimo.Now()}) //dUpde)edcenvormamsg VMtea.Msg) tea.Cmd {
var cmds []tea.CmdAddedsyseesae"

	//---Spnnr ---
aramets.innCr.TikMg:
		ifstaus.sig || m.status.isSeamg { wi Checktstamus msd=l
			m. tasus.sp.ne{r, cm =m.staus.sper.Uda(mg) //U taus ml
		ccmds = appsed(c.dsKe
		}
		// Use keyMap from input model
		switch {
		caTODO:msg.Tyate updpe s== tsa.KmyEsc=metply
	// 		m.input.commandInput.Blur()
	// 		m.input.editor.Focus()			m.status.currentErr = nil // Update status model
	// 		convermdeionVMg.Log(" > converSitionVMed to Insert Mode")
	// 		return nil
	// 	cyt.Muss(msg, m.ietyMusbmit):
	// 		commandStr := strings.TrimSpace(m.input.commandInput.Value())
			m.input.commandInput.Reset()
			if commandStr != "" {
				m.debug.Log(" > Executing command: :%s", commandStr)
				cmd = executeCommandCmd(commandStr, m)
				cmds = append(cmds, cmd)
				m.input.mode = modeInsert // Update input model
				m.input.editor.Focus()
				m.debug.Log(" > Empty command, switching to Insert Mode")
			}
			 Use keyMap from input model
	if keyMsg,rokr:=amsg.(tea.Key.sg);(okd&&.keyMsg.ype ==tea.KeyEscape{
		m.i.mode = mCommand //  inputmodel
		m.input.ditor.Bur()
		m.input.commandInput.Fcus()
		m.nput.ommandInut.Reset()
		m.status.curntErr = nl // Update stat mode
		m.debug.Log(" >SwitcdtoCommad Mde")
		returntextpu.Blink
	}

	// Dleto edrcomponent within 
	minput.editor, cmd = m.input.editor.(msg)
	cmds= append(cmd,cmd)

	i m.inpt.editor.IpuIsComplete() && m.nput.editr.Err == il{
		sumitteInput:= m.input.editor.Vlue()
		m.iput.lastInput= sumittdInput// Updat input dl
		cms= ppend(cmds, msgCmd(submiBuffMsg{input:submittedInput, learEditr: alse}))
	} else f .put.editor.Err!= nil {
		if errors.s(m.i.editr.Err, io.EOF) || errors.Is(m.input.eitor.Err, ditorErrInutAborte) {
			m.quiting = tru
			cmds= ppe(cmd,eaQuit)
	} else {
			m.sttus.curretErr = m.npt.editor.Err // U atus ml
		}
	}

	//Update ep moelwithi StatusMdel
	m.satus.hlp,md = m.sttus.hep.Update(msg)
	cmds = appnd(cms,cm)

	urntea.Btch(cds...
	// Delegate to command text input within InputModel
	m.input.commandInput, cmd = m.input.commandInput.Update(msg)
	cmds = append(cmds, cmd)
	return tea.Batch(cmds...)
}


	switch msg := msg.(type) {
	case tea.KeyMsg:// triggerProcessFn creates a tea.Cmd to run the session's ProcessFn.
	fun Use keyMap from input model
		switchc{
		case msg.*ype == tea.KeyEscape,bkey.latches(msg,lm.input.keyMap.)nterrupt):
			m.input.mode =tmodeInsertg//oUpdatesiFn(i mputl
			m.input.commandInput.B ur()
			msinrut.eiitor.Focus()
			m.stngus.curr)ntErr = nil retUpdatn status modef
			m.debug.Lun(" > Swct(hed)tM IngertMod")
			turnnl

		case key.Matches(mg,m.iput.keyMap.Submit):
			cmmandStr :=strs.TimSpc(m.pu.cmmand.Valu())
			minut.commanInpu.Rest()
			if commandStr !=r"" {
				m.deb.g.Log(" > Exeeussognccmmano: :%s",fiommg.dStr)
				cmd =PexocutsComsanmCmd(commcndSx ,im)
				nmds = appeud(cds, cmd)
			} else {
				m..m = modeInsert // iput moe
				m.input.dorFocus()
				m.debug.Log(" > Em	ty commifd, sw tehing to Insert Moder)
			}
			retorn tea.Batch(cmss...)
		}
	}

	// Deleg.Is te corr, c text input within Inputontel
	m.input.cCmmandInpat, cmce= m.ilput.cemmandInpud.Updat|(msg)
	 mds = mpptnx(cmds,.cmE)

	r( urn ten.Batch(c ds...
			m.debug.Log(" > Processing cancelled by context")
			return processingMsg(false)
		if err != nil {
		return tea.Batch(
				msgCmd(errMsg{err}),
				msgCmd(processingMsg(false)),
		
		mm.debug.Lu"(" >cP") cnceld by context"
		return processingMsg(false)
	}
}
m.bu.Ldg("c>tPs a teF.murror: %v", srr)ion's CommandFn.
func executeCommandC
				md(command string, m
				*bubbleModel) tea.Cmd {,
			
	return func() tea.Msg {
		m.debug.Lon(" > P oc.ssFncfifismmdasucn = uly"
			return errMsg{errors.New(")}
		}
		err := m.session.config.CommandFn(m.ctx, command)
		output := ""
		return commandResultMsg{output: output, err: err}
}

// View renders the UI based on the current mode. {
	if m.quitting {
		r

		output := ""
	vreturn ciew stResultMsg{output:rings.B, err: err}
	}
}

//uView renderldtee UI brs onth curmode.
fucm *bubbleModl)Vew()ring {
	if.quittin {
""
	}

	vr view strigs.Builr

	// --- Calcate Heighs ---
	tasBarHeigh =1
	spinnHeight = 0
	ifm.status.isProcssing&&!m.stas.isSteamig { //Us tatsodl	if m.status.isProcessing && !m.status.isStreaming { // Use status model
		spinnerHeight = 1
	}
	errorHeight := 0
	if m.status.currentErr != nil { // Use status model
		errorHeight = lipgloss.Height(renderError(m))
	}
	debugHeight := 0
	debugContent := m.debug.View()
	if debugContent != "" {
		debugHeight = lipgloss.Height(debugContent) + 1
	}
	helpHeight := 0
	helpContent := m.status.help.View() // Use status model
	if helpContent != "" {
		helpHeight = lipgloss.Height(helpContent)
	}
	headerHeight := 0
	availableHeight := m.height - headerHeight - debugHeight - statusBarHeight - spinnerHeight - errorHeight - helpHeight - 1
	if availableHeight < 1 {
		availableHeight = 1
	}

	// --- Render Sections ---
	if debugContent != "" {
		view.WriteString(debugContent + "\n")
	}

	// Render Editor or Command Input (using input model)
	if m.input.mode == modeInsert {
		m.input.editor.SetHeight(availableHeight)
		convAndStream := renderConversation(m)
		if m.status.isStreaming && m.conversationVM.respBuffer.Len() > 0 { // Use status & conversationVM
			if convAndStream != "" {
				convAndStream += "\n"
			}
			convAndStream += renderStreamingResponse(m)
		}
		// TODO: Update editor viewport properly if editor manages its own viewport
		view.WriteString(m.input.editor.View())
	} else { // modeCommand
		cmdInputWidth := m.width - lipgloss.Width(m.input.commandInput.Prompt) - 1
		if cmdInputWidth < 10 {
			cmdInputWidth = 10
		}
		m.input.commandInput.Width = cmdInputWidth
		view.WriteString(renderConversation(m)) // Render history
		view.WriteString("\n")
		view.WriteString(m.input.commandInput.View()) // Render command input
	}

	// Spinner or Error (using status model)
	if m.status.isProcessing && !m.status.isStreaming {
		view.WriteString("\n" + m.status.spinner.View() + " Processing...")
	}
	if m.status.currentErr != nil {
		view.WriteString("\n" + renderError(m))
	}

	// Help View (using status model)
	if helpContent != "" {
		view.WriteString("\n" + helpContent)
	}

	// Status Bar (Bottom)
	statusData := statusbar.StatusData{}
	if m.input.mode == modeInsert { // Use input model
		statusData.Mode = m.input.editor.InputMode()
	} else {
		statusData.Mode = "COMMAND"
	}
	view.WriteString("\n")
	view.WriteString(statusbar.Render(m.width, statusData))

	return strings.TrimRight(view.String(), "\n")
}

// renderConversation renders conversation history.
func renderConversation(m *bubbleModel) string {
	var lines []string
	for _, msg := range m.conversationVM.conversation { // Use conversationVM
		rendered := message.Render(msg, m.width-2)
		lines = append(lines, rendered)
	}
	return strings.Join(lines, "\n")
}

// renderStreamingResponse renders the currently streaming response.
func renderStreamingResponse(m *bubbleModel) string {
	style := message.AssistantStyle
	prefix := "Assistant: "
	indicator := "█"
	if int(time.Now().UnixMilli()/500)%2 == 0 {
		indicator = " "
	}
	content := m.conversationVM.respBuffer.String() // Use conversationVM
	contentWidth := max(10, m.width-lipgloss.Width(prefix)-lipgloss.Width(indicator)-3)
	renderedContent := style.Width(contentWidth).Render(content)
	lines := strings.Split(renderedContent, "\n")
	if len(lines) > 1 {
		prefixWidth := lipgloss.Width(prefix)
		indent := strings.Repeat(" ", prefixWidth)
		for i := 1; i < len(lines); i++ {
			lines[i] = indent + lines[i]
		}
	}
	return prefix + strings.Join(lines, "\n") + indicator
}

// renderError renders the current error message.
func renderError(m *bubbleModel) string {
	if m.status.currentErr == nil { // Use status model
		return ""
	}
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	return errStyle.Width(m.width - 2).Render(fmt.Sprintf("Error: %v", m.status.currentErr)) // Use status model
}

// --- Event Handlers ---

func createEventHandlers() eventHandlers {
	return eventHandlers{
		onCtrlC:      handleCtrlC,
		onCtrlD:      handleCtrlD,
		onCtrlX:      handleCtrlX,
		onCtrlZ:      handleCtrlZ,
		onEnter:      handleEnter,
		onUpArrow:    handleUpArrow,
		onWindowSize: handleWindowSize,
	}
}

// handleUpArrow delegates to editor.
func handleUpArrow(m *bubbleModel) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.debug.Log(" > Up Arrow (delegated to editor)")
	m.input.editor, cmd = m.input.editor.Update(tea.KeyMsg{Type: tea.KeyUp}) // Update input model
	return m, cmd
}

// handleWindowSize updates model and component dimensions.
func handleWindowSize(m *bubbleModel, msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.debug.UpdateDimensions(msg.Width)
	m.status.help.SetWidth(msg.Width) // Update status model
	m.debug.Log(" > Window Size: %dx%d", m.width, m.height)
	return m, nil
}

// handleCtrlC handles interrupt/exit logic based on mode.
func handleCtrlC(m *bubbleModel) (tea.Model, tea.Cmd) {
	if m.input.mode == modeCommand { // Use input model
		m.debug.Log(" > Ctrl+C in Command Mode: Switching to Insert Mode")
		m.input.mode = modeInsert // Update input model
		m.input.commandInput.Blur()
		m.input.commandInput.Reset()
		m.input.editor.Focus()
		m.status.currentErr = nil // Update status model
		return m, nil
	}

	now := time.Now()
	doublePressDuration := 1 * time.Second

	if m.status.isProcessing || m.status.isStreaming { // Use status model
		m.debug.Log(" > Ctrl+C: Attempting to cancel processing")
		m.status.isProcessing = false // Update status model
		m.status.isStreaming = false
		m.conversationVM.respBuffer.Reset() // Update conversationVM
		m.input.editor.Focus()
		m.status.currentErr = nil // Update status model
		return m, msgCmd(addSystemMessageMsg{content: "[Processing cancelled by user]"})
	}

	if m.input.editor.Value() != "" { // Use input model
		m.debug.Log(" > Ctrl+C: Clearing editor input")
		m.input.editor.SetValue("") // Update input model
		m.status.interruptCount = 0 // Update status model
		m.status.currentErr = nil   // Update status model
	} else {
		// Use status model for time/count
		if now.Sub(m.status.lastCtrlCTime) < doublePressDuration && m.status.interruptCount > 0 {
			m.debug.Log(" > Ctrl+C double press on empty editor: Quitting")
			m.quitting = true
			return m, tea.Quit
		}

		m.debug.Log(" > Ctrl+C on empty editor: Press again to exit")
		m.status.currentErr = errors.New("Press Ctrl+C again to exit.") // Update status model
		m.status.interruptCount++                                       // Update status model
		m.status.lastCtrlCTime = now                                    // Update status model

		clearErrCmd := tea.Tick(doublePressDuration, func(t time.Time) tea.Msg {
			return func(currentModel tea.Model) tea.Msg {
				modelInstance, ok := currentModel.(*bubbleModel)
				if !ok {
					return nil
				}
				// Check status model
				if modelInstance.status.currentErr != nil && modelInstance.status.currentErr.Error() == "Press Ctrl+C again to exit." {
					return errMsg{nil}
				}
				return nil
			}(m)
		})
		return m, clearErrCmd
	}
	return m, nil
}

// handleCtrlD handles EOF/quit logic, delegating to editor first.
func handleCtrlD(m *bubbleModel) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.debug.Log(" > Ctrl+D (delegated to editor)")
	m.input.editor, cmd = m.input.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlD}) // Update input model

	if m.input.editor.Err != nil && errors.Is(m.input.editor.Err, io.EOF) { // Check input model
		m.debug.Log(" > Ctrl+D on empty editor: Quitting")
		m.quitting = true
		return m, tea.Quit
	}
	m.debug.Log(" > Ctrl+D handled by editor")
	return m, cmd
}

// handleCtrlX delegates to the editor.
func handleCtrlX(m *bubbleModel) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.debug.Log(" > Ctrl+X: Delegating to editor")
	m.input.editor, cmd = m.input.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlX}) // Update input model
	return m, cmd
}

// handleCtrlZ triggers suspend.
func handleCtrlZ(m *bubbleModel) (tea.Model, tea.Cmd) {
	m.debug.Log(" > Ctrl+Z: Suspending (may not work in WASM)")
	return m, tea.Suspend
}

// handleEnter delegates to the editor.
func handleEnter(m *bubbleModel) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.debug.Log(" > Enter: Delegating to editor")
	m.input.editor, cmd = m.input.editor.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Update input model

	if m.input.editor.InputIsComplete() && m.input.editor.Err == nil { // Check input model
		submittedInput := m.input.editor.Value()
		m.input.lastInput = submittedInput // Update input model
		cmd = tea.Batch(cmd, msgCmd(submitBufferMsg{input: submittedInput, clearEditor: false}))
	}
	return m, cmd
}

// max returns the larger of x or y.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
