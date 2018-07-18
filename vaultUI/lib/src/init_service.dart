import 'dart:async';
import 'dart:convert';
import 'dart:html';
import 'package:angular/angular.dart';

@Injectable()
class InitService{
  //String _log;
  final HttpRequest _request;
  String _host;   // Vault addreess
  String _authToken;

  InitService(this._request) {
    _host = window.location.origin;
    _authToken = window.localStorage['Token'] == null ? '' : window.localStorage['Token'];
  }

  Future<Map<String, dynamic>> MakeRequest(Map<String, dynamic> request) async{
    String url = _host + '/twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/InitVault';
    Completer<Map<String,dynamic>> response = new Completer<Map<String, dynamic>>();
    try {
      _request.open('POST', url);
      _request.setRequestHeader('Content-Type', 'application/json');
      _request.setRequestHeader('Authorization', _authToken);
      _request.send(json.encode(request));
      _request.onLoadEnd.listen((_) {
        if (_request.status == 401) { // Unauthorized, return error to caller

        }
        Map<String, dynamic> responseJSON = json.decode(_request.responseText);
        if(responseJSON['success']) {
          response.complete({
            'log': utf8.decode(base64Decode(responseJSON['logfile'])),
            'tokens': responseJSON['tokens']
          });
        } else {
          print('failure!');
        }
      });
      return response.future;
    } catch(err) {
      print(err);
    }
    response.complete({
            'log': 'Error in reading logs',
            'tokens': ''
          });
    return response.future;
  }
}
