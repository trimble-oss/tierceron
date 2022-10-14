//go:build argosy && tc
// +build argosy,tc
 
package argosyopts
 
import (
   "encoding/json"
   "log"
   //"fmt"
   "math"
   "strconv"
   "tierceron/trcvault/util"
   "tierceron/vaulthelper/kv"
 
   tcbuildopts "VaultConfig.TenantConfig/util/buildopts"
   "github.com/mrjrieke/nute/mashupsdk"
)
 
//var maxTime int64
 
var elementcollectionIDs []int64
var curvecollectionIDs []int64
 
func GetStubbedDataFlowStatistics() ([]string, map[string][]float64) {
   //  return data, TimeData
   return tcbuildopts.GetStubbedDataFlowStatistics()
}
 
func getGroupSize(groups []util.TTDINode) (float64, float64, float64) {
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
 
func buildArgosies(startID int64, args util.TTDINode) (util.TTDINode, []int64, []int64) {
   var argosyId int64
   //collectionIDs := []int64{}
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
           Name:           name,
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
       //collection := []int64{}
       children := []int64{}
 
       argosy.ChildNodes, _, children, curveCollection, argosy = buildDataFlowGroups(argosyId+1, argosy, dfgsize, dfsize, dfstatsize, argosyId)
       //collectionIDs = append(collectionIDs, collection...)
       argosy.MashupDetailedElement.Childids = append(argosy.MashupDetailedElement.Childids, children...)
       curveIds = append(curveIds, curveCollection...)
 
       args.ChildNodes[i] = argosy
   }
   return args, elementcollectionIDs, curveIds
}
 
func buildDataFlowGroups(startID int64, argosy util.TTDINode, dfgsize float64, dfsize float64, dfstatsize float64, parentID int64) ([]util.TTDINode, []int64, []int64, []int64, util.TTDINode) {
   argosyId := startID - 1
   //collectionIDs := []int64{}
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
           Name:           name,
           Alias:          "It",
           Description:    "",
           Data:           data,
           Custosrenderer: "",
           Renderer:       "Element",
           Genre:          "DataFlowGroup",
           Subgenre:       "",
           Parentids:      []int64{parentID},
           Childids:       []int64{-2},
       }
       //collection := []int64{}
       children := []int64{}
 
       group.ChildNodes, _, children, curveCollection, group = buildDataFlows(argosyId+1, group, dfsize, dfstatsize, argosyId)
       // for _, id := range collection {
       //  collectionIDs = append(collectionIDs, id)
       // }
       for _, id := range children {
           group.MashupDetailedElement.Childids = append(group.MashupDetailedElement.Childids, id)
       }
       argosy.ChildNodes[i] = group
   }
   return argosy.ChildNodes, elementcollectionIDs, childIDs, curveCollection, argosy
}
 
func buildDataFlows(startID int64, group util.TTDINode, dfsize float64, dfstatsize float64, parentID int64) ([]util.TTDINode, []int64, []int64, []int64, util.TTDINode) {
   argosyId := startID - 1
   //collectionIDs := []int64{}
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
           Name:           name, //"DataFlow-" + strconv.Itoa(int(argosyId)),
           Alias:          "It",
           Description:    "",
           Data:           data,
           Custosrenderer: "",
           Renderer:       "Element",
           Genre:          "DataFlow",
           Subgenre:       "",
           Parentids:      []int64{parentID},
           Childids:       []int64{-2},
       }
       //otherIds := []int64{}
       children := []int64{}
 
       flow.ChildNodes, _, children, curveCollection, flow = buildDataFlowStatistics(argosyId+1, flow, dfstatsize, argosyId)
       // for _, id := range otherIds {
       //  collectionIDs = append(collectionIDs, id)
       // }
       for _, id := range children {
           flow.MashupDetailedElement.Childids = append(flow.MashupDetailedElement.Childids, id)
       }
       group.ChildNodes[i] = flow
   }
   return group.ChildNodes, elementcollectionIDs, childIDs, curveCollection, group
}
 
func buildDataFlowStatistics(startID int64, flow util.TTDINode, dfstatsize float64, parentID int64) ([]util.TTDINode, []int64, []int64, []int64, util.TTDINode) {
   argosyId := startID - 1
   //collectionIDs := []int64{}
   childIDs := []int64{}
   curveCollection := []int64{}
   //total := int64(0)
   for i := 0; i < len(flow.ChildNodes); i++ {
       argosyId = argosyId + 1
       curvecollectionIDs = append(curvecollectionIDs, argosyId)
       childIDs = append(childIDs, argosyId)
       curveCollection = append(curveCollection, argosyId)
       stat := flow.ChildNodes[i]
       //name := stat.MashupDetailedElement.Name
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
       //total = int64(total) + int64(decodedStatData["TimeSplit"].(float64))
       // if int64(decodedStatData["TimeSplit"].(float64)) > maxTime {
       //  maxTime = int64(decodedStatData["TimeSplit"].(float64))
       // }
       stat.MashupDetailedElement = mashupsdk.MashupDetailedElement{
           Id:             argosyId,
           State:          &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Hidden)},
           Name:           decodedStatData["StateName"].(string) + "-" + strconv.Itoa(int(argosyId)), //"DataFlowStatistic-" + strconv.Itoa(int(argosyId)), //data[pointer], //
           Alias:          "It",
           Description:    "",
           Data:           data, //strconv.FormatInt(int64(decodedStatData["TimeSplit"].(float64)), 10), //time in nanoseconds
           Custosrenderer: "",
           Renderer:       "Curve",
           Genre:          "DataFlowStatistic",
           Subgenre:       "",
           Parentids:      []int64{parentID},
           Childids:       []int64{-1},
       }
       flow.ChildNodes[i] = stat
   }
   return flow.ChildNodes, elementcollectionIDs, childIDs, curveCollection, flow //, int64(total)
}
 
func BuildFleet(mod *kv.Modifier, logger *log.Logger) (util.TTDINode, error) {
   if mod == nil {
       return BuildStubFleet(mod, logger)
   }
 
   argosies := []util.TTDINode{
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
           []util.TTDINode{},
       },
       // {
       //  mashupsdk.MashupDetailedElement{
       //      Basisid:        -4,
       //      State:          &mashupsdk.MashupElementState{Id: -4, State: int64(mashupsdk.Hidden)},
       //      Name:           "{0}-SubSpiral",
       //      Alias:          "It",
       //      Description:    "",
       //      Data:           "",
       //      Custosrenderer: "",
       //      Renderer:       "Element",
       //      Genre:          "Solid",
       //      Subgenre:       "Ento",
       //      Parentids:      []int64{-2},
       //      Childids:       []int64{},
       //  },
       //  []util.TTDINode{},
       // },
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
           []util.TTDINode{},
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
           []util.TTDINode{},
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
           []util.TTDINode{},
       },
   }
   args, err := util.InitArgosyFleet(mod, "TenantDatabase", logger)
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
       return util.TTDINode{}, err
   }
   elementCollection := []int64{}
   //curveCollection := []int64{}
   args, elementCollection, _ = buildArgosies(8, args)
   elementCollection = elementcollectionIDs
   //args.Argosies = append(args.Argosies, argosies)
   //argosies = append(argosies, args)
   argosies = append(argosies, util.TTDINode{
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
       []util.TTDINode{},
   })
   curvecollectionIDs = append(curvecollectionIDs, 1)
   argosies = append(argosies, util.TTDINode{
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
       []util.TTDINode{},
   })
  
  
   args.ChildNodes = append(args.ChildNodes, argosies...)
   // args.MashupDetailedElement = mashupsdk.MashupDetailedElement{
   //  Id:             7,
   //  State:          &mashupsdk.MashupElementState{Id: 7, State: int64(mashupsdk.Init)},
   //  Name:           "TenantDatabase",
   //  Description:    "",
   //  Data:           "",
   //  Custosrenderer: "",
   //  Renderer:       "",
   //  Colabrenderer:  "",
   //  Genre:          "Collection",
   //  Subgenre:       "Element",
   //  Parentids:      nil,
   //  Childids:       []int64{},
   // }
   // //args.MashupDetailedElement.Childids = append(args.MashupDetailedElement.Childids, curveCollection...)
   // args.MashupDetailedElement.Childids = append(args.MashupDetailedElement.Childids, elementCollection...)
   // check := false
   //  //ids := []int64{}
   //  for id := range args.MashupDetailedElement.Childids {
   //      if id == 8 {
   //          check = true
   //      }
   //      // for j := 0; j < len(ids); j++ {
   //      //  if int64(id) == int64(ids[j])  {
   //      //      check = true
   //      //  }
              
   //      // }
   //      // ids = append(ids, int64(id))
   //  }
      
   //  fmt.Println(check)
   return args, nil
}
 
func GetDataFlowGroups(mod *kv.Modifier, argosy *util.TTDINode) []util.TTDINode {
   return nil
}
