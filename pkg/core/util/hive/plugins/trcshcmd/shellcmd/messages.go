package shellcmd

// Shell command types sent via ChatMsg.Response field
const (
	CmdTrcConfig  = "tconfig"
	CmdTrcPub     = "tpub"
	CmdTrcSub     = "tsub"
	CmdTrcX       = "tx"
	CmdTrcInit    = "tinit"
	CmdTrcPlgtool = "trcplgtool"
	CmdKubectl    = "kubectl"
	CmdTrcBoot    = "tboot"
	CmdRm         = "rm"
	CmdCp         = "cp"
	CmdMv         = "mv"
	CmdCat        = "cat"
	CmdSu         = "su"
)
