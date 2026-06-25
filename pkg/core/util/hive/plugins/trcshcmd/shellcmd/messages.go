package shellcmd

// Shell command types sent via ChatMsg.Response field
const (
	CmdTrcConfig  = "tconfig"
	CmdTrcPub     = "tpub"
	CmdTrcSub     = "tsub"
	CmdTrcX       = "tx"
	CmdTrcInit    = "tinit"
	CmdTrcPlgtool = "trcplgtool"
	CmdTrcTv      = "tv"
	CmdKubectl    = "kubectl"
	CmdTrcBoot    = "tboot"
	CmdRm         = "rm"
	CmdCp         = "cp"
	CmdMv         = "mv"
	CmdCat        = "cat"
	CmdMkdir      = "mkdir"
	CmdNano       = "rosea"
	CmdSu         = "su"
)
