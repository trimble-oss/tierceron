//go:build argosy && tc
// +build argosy,tc
 
package argosyopts
 
import (
   "encoding/json"
   "log"
   "math"
   "strconv"
   "tierceron/trcvault/flowutil"
   "tierceron/vaulthelper/kv"
 
   tcbuildopts "VaultConfig.TenantConfig/util/buildopts"
   "github.com/mrjrieke/nute/mashupsdk"
)
 
var elementcollectionIDs []int64
var curvecollectionIDs []int64
 
func GetStubbedDataFlowStatistics() ([]string, map[string][]float64) {
   return tcbuildopts.GetStubbedDataFlowStatistics()
}
 
func getGroupSize(groups []flowutil.TTDINode) (float64, float64, float64) {
   groupsize := 0.0
   flowsize := 0.0
   statsize := 0.0
   for _, group := range groups {
       groupsize += float64(len(group.ChildNodes))
       for _, flow := range group.ChildNodes {
           flowsize += float64(len(flow.ChildNodes))
           statsize = float64(len(flow.ChildNodes))
       }
   }
   return groupsize, flowsize, statsize
}
 
func buildArgosies(startID int64, args flowutil.TTDINode) (flowutil.TTDINode, []int64, []int64) {
   var argosyId int64
   curveCollection := []int64{}
   curveIds := []int64{}
   for i := 0; i < len(args.ChildNodes); i++ {
       dfgsize, dfsize, dfstatsize := getGroupSize(args.ChildNodes[i].ChildNodes)
       argosyId = startID + int64(i)*int64(1.0+float64(dfgsize)+math.Pow(float64(dfsize), 2.0)+math.Pow(float64(dfstatsize), 3.0))
       elementcollectionIDs = append(elementcollectionIDs, argosyId)
       argosy := args.ChildNodes[i]
       name := argosy.MashupDetailedElement.Name
       data := argosy.MashupDetailedElement.Data
       argosy.MashupDetailedElement = mashupsdk.MashupDetailedElement{
           Id:             argosyId,
           State:          &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Init)},
           Name:           name + "-" + strconv.Itoa(int(argosyId)),
           Alias:          "It",
           Description:    "Testing to see if description will properly change!",
           Data:           data,
           Custosrenderer: "TenantDataRenderer",
           Renderer:       "Element",
           Genre:          "Argosy",
           Subgenre:       "",
           Parentids:      []int64{},
           Childids:       []int64{-2},
       }
       children := []int64{}
 
       argosy.ChildNodes, _, children, curveCollection, argosy = buildDataFlowGroups(argosyId+1, argosy, dfgsize, dfsize, dfstatsize, argosyId)

       argosy.MashupDetailedElement.Childids = append(argosy.MashupDetailedElement.Childids, children...)
       curveIds = append(curveIds, curveCollection...)
 
       args.ChildNodes[i] = argosy
   }
   return args, elementcollectionIDs, curveIds
}
 
func buildDataFlowGroups(startID int64, argosy flowutil.TTDINode, dfgsize float64, dfsize float64, dfstatsize float64, parentID int64) ([]flowutil.TTDINode, []int64, []int64, []int64, flowutil.TTDINode) {
   argosyId := startID - 1
   childIDs := []int64{}
   curveCollection := []int64{}
   for i := 0; i < len(argosy.ChildNodes); i++ {
       argosyId = startID + int64(i)*int64(1.0+float64(dfsize)+math.Pow(float64(dfstatsize), 2.0))
       elementcollectionIDs = append(elementcollectionIDs, argosyId)
       childIDs = append(childIDs, argosyId)
       group := argosy.ChildNodes[i]
       name := group.MashupDetailedElement.Name
       data := group.MashupDetailedElement.Data
       group.MashupDetailedElement = mashupsdk.MashupDetailedElement{
           Id:             argosyId,
           State:          &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Hidden)},
           Name:           name + "-" + strconv.Itoa(int(argosyId)),
           Alias:          "It",
           Description:    "",
           Data:           data,
           Custosrenderer: "TenantDataRenderer",
           Renderer:       "Element",
           Genre:          "DataFlowGroup",
           Subgenre:       "",
           Parentids:      []int64{parentID},
           Childids:       []int64{-2},
       }
       children := []int64{}
 
       group.ChildNodes, _, children, curveCollection, group = buildDataFlows(argosyId+1, group, dfsize, dfstatsize, argosyId)

       for _, id := range children {
           group.MashupDetailedElement.Childids = append(group.MashupDetailedElement.Childids, id)
       }
       argosy.ChildNodes[i] = group
   }
   return argosy.ChildNodes, elementcollectionIDs, childIDs, curveCollection, argosy
}
 
func buildDataFlows(startID int64, group flowutil.TTDINode, dfsize float64, dfstatsize float64, parentID int64) ([]flowutil.TTDINode, []int64, []int64, []int64, flowutil.TTDINode) {
   argosyId := startID - 1
   childIDs := []int64{}
   curveCollection := []int64{}
   for i := 0; i < len(group.ChildNodes); i++ {
       argosyId = startID + int64(i)*int64(1.0+float64(dfstatsize))
       elementcollectionIDs = append(elementcollectionIDs, argosyId)
       childIDs = append(childIDs, argosyId)
       flow := group.ChildNodes[i]
       name := flow.MashupDetailedElement.Name
       data := flow.MashupDetailedElement.Data
       flow.MashupDetailedElement = mashupsdk.MashupDetailedElement{
           Id:             argosyId,
           State:          &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Hidden)},
           Name:           name + "-" + strconv.Itoa(int(argosyId)), 
           Alias:          "It",
           Description:    "",
           Data:           data,
           Custosrenderer: "TenantDataRenderer",
           Renderer:       "Element",
           Genre:          "DataFlow",
           Subgenre:       "",
           Parentids:      []int64{parentID},
           Childids:       []int64{-2},
       }
       children := []int64{}
 
       flow.ChildNodes, _, children, curveCollection, flow = buildDataFlowStatistics(argosyId+1, flow, dfstatsize, argosyId)

       for _, id := range children {
           flow.MashupDetailedElement.Childids = append(flow.MashupDetailedElement.Childids, id)
       }
       group.ChildNodes[i] = flow
   }
   return group.ChildNodes, elementcollectionIDs, childIDs, curveCollection, group
}
 
func buildDataFlowStatistics(startID int64, flow flowutil.TTDINode, dfstatsize float64, parentID int64) ([]flowutil.TTDINode, []int64, []int64, []int64, flowutil.TTDINode) {
   argosyId := startID - 1
   childIDs := []int64{}
   curveCollection := []int64{}
   for i := 0; i < len(flow.ChildNodes); i++ {
       argosyId = argosyId + 1
       curvecollectionIDs = append(curvecollectionIDs, argosyId)
       childIDs = append(childIDs, argosyId)
       curveCollection = append(curveCollection, argosyId)
       stat := flow.ChildNodes[i]
       data := stat.MashupDetailedElement.Data
       var decodedstat interface{}
       err := json.Unmarshal([]byte(data), &decodedstat)
       if err != nil {
           log.Println("Error in decoding data in buildDataFlowStatistics")
           break
       }
       decodedStatData := decodedstat.(map[string]interface{})
       if decodedStatData["TimeSplit"] == nil || decodedStatData["StateName"] == nil {
           log.Println("Error in decoding data in buildDataFlowStatistics because data not initialized properly")
           break
       }
       stat.MashupDetailedElement = mashupsdk.MashupDetailedElement{
           Id:             argosyId,
           State:          &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Hidden)},
           Name:           decodedStatData["StateName"].(string) + "-" + strconv.Itoa(int(argosyId)), //"DataFlowStatistic-" + strconv.Itoa(int(argosyId)), //data[pointer], //
           Alias:          "It",
           Description:    "",
           Data:           data, 
           Custosrenderer: "TenantDataRenderer",
           Renderer:       "Curve",
           Genre:          "DataFlowStatistic",
           Subgenre:       "",
           Parentids:      []int64{parentID},
           Childids:       []int64{-1},
       }
       flow.ChildNodes[i] = stat
   }
   return flow.ChildNodes, elementcollectionIDs, childIDs, curveCollection, flow 
}
 
func BuildFleet(mod *kv.Modifier, logger *log.Logger) (flowutil.TTDINode, error) {
   if mod == nil {
       return BuildStubFleet(mod, logger)
   }
 
   argosies := []flowutil.TTDINode{
       {
           mashupsdk.MashupDetailedElement{
               Id:             3,
               State:          &mashupsdk.MashupElementState{Id: 3, State: int64(mashupsdk.Init)},
               Name:           "Outside",
               Alias:          "Outside",
               Description:    "The background was selected",
               Data:           "",
               Custosrenderer: "",
               Renderer:       "Background",
               Genre:          "Space",
               Subgenre:       "Exo",
               Parentids:      nil,
               Childids:       nil,
           },
           []flowutil.TTDINode{},
       },
       {
           mashupsdk.MashupDetailedElement{
               Basisid:        -1,
               State:          &mashupsdk.MashupElementState{Id: -1, State: int64(mashupsdk.Mutable)},
               Name:           "Curve",
               Alias:          "It",
               Description:    "",
               Data:           "",
               Custosrenderer: "",
               Renderer:       "Curve",
               Colabrenderer:  "Element",
               Genre:          "Solid",
               Subgenre:       "Skeletal",
               Parentids:      []int64{},
               Childids:       []int64{},
           },
           []flowutil.TTDINode{},
       },
       {
           mashupsdk.MashupDetailedElement{
               Id:             1,
               State:          &mashupsdk.MashupElementState{Id: 1, State: int64(mashupsdk.Init)},
               Name:           "CurvePathEntity-1",
               Description:    "",
               Data:           "",
               Custosrenderer: "",
               Renderer:       "Curve",
               Colabrenderer:  "Element",
               Genre:          "Solid",
               Subgenre:       "Skeletal",
               Parentids:      []int64{},
               Childids:       []int64{-1},
           },
           []flowutil.TTDINode{},
       },
       {
           mashupsdk.MashupDetailedElement{
               Basisid:        -2,
               State:          &mashupsdk.MashupElementState{Id: -2, State: int64(mashupsdk.Mutable)},
               Name:           "{0}-Path",
               Alias:          "It",
               Description:    "Path was selected",
               Data:           "",
               Custosrenderer: "",
               Renderer:       "Element",
               Genre:          "Solid",
               Subgenre:       "Ento",
               Parentids:      []int64{},
               Childids:       []int64{},
           },
           []flowutil.TTDINode{},
       },
   }
   args, err := flowutil.InitArgosyFleet(mod, "TenantDatabase", logger)
   var count int
   for _, arg := range args.ChildNodes {
       count++
       for _, dfg := range arg.ChildNodes {
           count++
           for _, df := range dfg.ChildNodes {
               count++
               count += len(df.ChildNodes)
           }
       }
   }
   if err != nil {
       return flowutil.TTDINode{}, err
   }
   elementCollection := []int64{}
   args, elementCollection, _ = buildArgosies(8, args)
   elementCollection = elementcollectionIDs
   argosies = append(argosies, flowutil.TTDINode{
       mashupsdk.MashupDetailedElement{
           Id:             4,
           State:          &mashupsdk.MashupElementState{Id: 4, State: int64(mashupsdk.Init)},
           Name:           "PathGroupOne",
           Description:    "Paths",
           Data:           "",
           Custosrenderer: "",
           Renderer:       "Element",
           Genre:          "Collection",
           Subgenre:       "Element",
           Parentids:      []int64{},
           Childids:       elementCollection,
       },
       []flowutil.TTDINode{},
   })
   curvecollectionIDs = append(curvecollectionIDs, 1)
   argosies = append(argosies, flowutil.TTDINode{
       mashupsdk.MashupDetailedElement{
           Id:             2,
           State:          &mashupsdk.MashupElementState{Id: 2, State: int64(mashupsdk.Init)},
           Name:           "CurvesGroupOne",
           Description:    "Curves",
           Data:           "",
           Custosrenderer: "",
           Renderer:       "Curve",
           Colabrenderer:  "Path",
           Genre:          "Collection",
           Subgenre:       "Skeletal",
           Parentids:      nil,
           Childids:       curvecollectionIDs,
       },
       []flowutil.TTDINode{},
   })
  
  
   args.ChildNodes = append(args.ChildNodes, argosies...)
   return args, nil
}
 
func GetDataFlowGroups(mod *kv.Modifier, argosy *flowutil.TTDINode) []flowutil.TTDINode {
   return nil
}
