package pluginlib

import (
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
)

func Init(pluginName string,
	properties *map[string]any,
	PostInit func(*tccore.ConfigContext),
) (*tccore.ConfigContext, error) {
	if properties == nil {
		fmt.Fprintln(os.Stderr, "Missing initialization components")
		return tccore.InitPost(pluginName, properties, PostInit)
func SendDfStat(configContext *tccore.ConfigContext, dfsctx *tccore.DeliverStatCtx, dfstat *tccore.TTDINode) {
	dfstat.Name = configContext.ArgosId
	dfstat.FinishStatistic("", "", "", configContext.Log, true, dfsctx)
	configContext.Log.Printf("Sending dataflow statistic to kernel: %s\n", dfstat.Name)
	dfstatClone := *dfstat
	go func(dsc *tccore.TTDINode) {
		if configContext != nil && *configContext.DfsChan != nil {
			*configContext.DfsChan <- dsc
		}
	}(&dfstatClone)
}
