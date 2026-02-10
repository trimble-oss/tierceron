package shellcmd

// Shell command types sent via ChatMsg.Response field
const (
	CmdTrcConfig  = "trcconfig"
	CmdTrcPub     = "trcpub"
	CmdTrcSub     = "trcsub"
	CmdTrcX       = "trcx"
	CmdTrcInit    = "trcinit"
	CmdTrcPlgtool = "trcplgtool"
	CmdKubectl    = "kubectl"
	CmdTrcBoot    = "trcboot"
	CmdRm         = "rm"
	CmdCp         = "cp"
	CmdMv         = "mv"
	CmdCat        = "cat"
	CmdSu         = "su"
)
