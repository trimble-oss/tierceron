import 'dart:async';
import 'dart:convert';
import 'dart:html';
import 'package:angular/angular.dart';

@Injectable()
class InitService{
  //String _log;
  final HttpRequest _request;
  final String _host = 'http://localhost:8008';   // Vault addreess
  final String _authToken = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIzMjE2NTQ5ODcwIiwibmFtZSI6IldlYiBBcHAiLCJpYXQiOjE1MTYyMzkwMjIsImlzcyI6IlZpZXdwb2ludCwgSW5jLiIsImF1ZCI6IlZpZXdwb2ludCBWYXVsdCBXZWJBUEkifQ.ls2cxzqIMv3_72BKH9K34uR-gWgeFTGqu-tXGh503Jg';

  InitService(this._request);

  Future<String> MakeRequest(Map<String, dynamic> request) async{
    String url = _host + "/twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/InitVault";
    Completer<String> response = new Completer<String>();
    try {
      _request.open('POST', url);
      _request.setRequestHeader("Content-Type", "application/json");
      _request.setRequestHeader("Authorization", _authToken);
      _request.send(json.encode(request));
      // final response = base64Decode(_log);
      // return utf8.decode(response);
      _request.onReadyStateChange.listen((_) {
        Map<String, dynamic> responseJSON = json.decode(_request.responseText);
        if(responseJSON["success"]) {
          print("success!");
          print(responseJSON["logfile"]);
          response.complete(utf8.decode(base64Decode(responseJSON["logfile"])));
        } else {
          print("failure!");
          response.complete("Init failed");
        }
      });
      return response.future;
    } catch(err) {
      print(err);
    }
  }
}
