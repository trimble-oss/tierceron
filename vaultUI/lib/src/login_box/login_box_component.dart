import 'package:angular/angular.dart';
import 'package:angular_forms/angular_forms.dart';
import 'package:angular_components/angular_components.dart';
import 'package:angular_router/angular_router.dart';

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
  @Input()
  String Username;
  @Input()
  String Password;

  @Input()
  bool IsSealed = true;
  @Input()
  String UnsealKey;
  Set<String> Keys = new Set();

  final Routes routes;
  LoginBoxComponent(this.routes);

  Future<Null> onActivate(_, RouterState current) async {
    IsSealed = current.queryParameters['sealed'].toLowerCase() == 'true';
    print(IsSealed);
  }

  final String _apiEndpoint = window.location.origin + '/twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/';   // Vault addreess

  Future<Null> SignIn() async{
    if (IsSealed) return;
    // Fetch input username/password for making the request.
    Map<String, dynamic> body = new Map();
    body['username'] = Username;
    body['password'] = Password;
    

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
        querySelector('#username').classes.addAll(['input_error', 'error_text']);
        querySelector('#password').classes.addAll(['input_error', 'error_text']);
        print('login failed');
      }
    }); 
    
    request.open('POST', _apiEndpoint + 'APILogin');
    request.setRequestHeader('Content-Type', 'application/json');
    request.send(json.encode(body));
  }  

  Future<Null> Unseal() async{
    if(UnsealKey == null || UnsealKey.length == 0){ // Check username exists
      querySelector('#unseal').classes.addAll(['input_error', 'error_text']);
      return;
    } 

    // Try to unseal with the key
    HttpRequest request = new HttpRequest();
    request.onLoadEnd.listen((_) {
      Map<String, dynamic> response = json.decode(request.response);
      if(request.status != 200) { // Unsucessful key
        Keys.add('UNSUCESSFUL: ' + UnsealKey);
        return;
      }
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
    request.send(json.encode({"unsealKey" : UnsealKey}));
  }

  // Remove error formatting from username/password box
  Future<Null> UnRedify(event) async {
    List<String> removals  = ['error', 'error_text'];
    (event.target as Element).classes.removeAll(removals);
  }

}