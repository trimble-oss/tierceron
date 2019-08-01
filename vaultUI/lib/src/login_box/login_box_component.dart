import 'package:angular/angular.dart';
import 'package:angular_forms/angular_forms.dart';
import 'package:angular_components/angular_components.dart';
import 'package:angular_router/angular_router.dart';
import 'package:http/browser_client.dart';

import '../routes.dart';
import 'dart:async';
import 'dart:html';
import 'dart:convert';

@Component(
  selector: 'login-box',
  styleUrls: ['login_box_component.css'],
  templateUrl: 'login_box_component.html',
  directives: const [coreDirectives,
                     formDirectives,
                     routerDirectives,
                     ModalComponent],
  providers: const [materialProviders, ClassProvider(Routes)]

)

class LoginBoxComponent implements OnActivate {
  List Envs;                           // Valid environment options
	
  @Input()
  String Username;
  @Input()
  String Password;
  @Input()
  String Environment = 'dev';

  @Input()
  bool IsSealed = true;
  @Input()
  String UnsealKey;
  Set<String> Keys = new Set();

  final Routes routes;
  LoginBoxComponent(this.routes);

  Future<Null> onActivate(_, RouterState current) async {
    IsSealed = current.queryParameters['sealed'].toLowerCase() == 'true';
    GetEnvironments(); //call GetEnvironments()
  }

  final String _apiEndpoint = window.location.origin + '/twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/';   // Vault addreess

  GetEnvironments() async {
    String valQuery = "twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/Environments";

    var client = new BrowserClient();
      var response =
          await client.post(valQuery, headers: {'Content-Type': 'application/json'}, body: '{}');
      Map respMap = json.decode(response.body);
      List environments = respMap['env'];
      Set envSet = Set();
      for(var i=0; i<environments.length; i++){
        envSet.add(environments[i]);
        if (environments[i] == 'dev' || environments[i] == 'prod') {
        	Environment = environments[i];
        }
      }
      Envs = envSet.toList();
  }
  
  Future<Null> SignIn() async{
    if (IsSealed) return;
    // Fetch input username/password for making the request.
    Map<String, dynamic> body = new Map();
    body['username'] = Username;
    body['password'] = Password;
    body['environment'] = Environment;

    // Construct request to twirp server
    HttpRequest request = new HttpRequest();
    request.onLoadEnd.listen((_) {
      body.remove('password'); // Clear password
      Password = '';
      Map<String, dynamic> response = json.decode(request.responseText);
      if(response['success'] != null && response['success']){
        print('login successful');
        window.localStorage['Token'] = response['authToken'];
        window.location.href = routes.values.toUrl();
        // Log in valid, proceed
      } else {
        querySelector('#username').classes.addAll(['input-error', 'error-text']);
        querySelector('#password').classes.addAll(['input-error', 'error-text']);
        querySelector('#warning').hidden=false;
        print('login failed');
      }
    }); 
    
    request.open('POST', _apiEndpoint + 'APILogin');
    request.setRequestHeader('Content-Type', 'application/json');
    request.send(json.encode(body));
  }  

  Future<Null> Unseal() async{
    if(UnsealKey == null || UnsealKey.length == 0){ // Check unseal exists
      querySelector('#unseal').classes.addAll(['input-error', 'error-text']);
      return;
    } 

    // Try to unseal with the key
    HttpRequest request = new HttpRequest();
    request.onLoadEnd.listen((_) {
      if(request.status != 200) { // Unsucessful key
        Keys.add('UNSUCESSFUL: ' + UnsealKey);
        return;
      }
      Map<String, dynamic> response = json.decode(request.response);
      if(response['sealed'] != null && response['sealed']) {
        int prog = response['progress'] == null ? 0 : response['progress'];
        int need = response['needed'] == null ? 0 : response['needed'];
        Keys.add(prog.toString() + '/' + need.toString() + ': ' +  UnsealKey);
      } else {
        IsSealed = false;
      }
    });

    request.open('POST', _apiEndpoint + 'Unseal');
    request.setRequestHeader('Content-Type', 'application/json');
    request.send(json.encode({'unsealKey' : UnsealKey}));
  }

  // Remove error formatting from username/password box
  Future<Null> UnRedify(event) async {
    List<String> removals  = ['input-error', 'error-text'];
    (event.target as Element).classes.removeAll(removals);
    querySelector('warning').hidden=true;
  }

}