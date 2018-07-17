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
    IsSealed = current.parameters['sealed'].toLowerCase() == 'true';
    print(IsSealed);
  }

  final String _apiEndpoint = window.location.origin + '/twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/';   // Vault addreess

  SignIn() {
    Map<String, dynamic> body = new Map();
    body['username'] = Username;
    body['password'] = Password;
    
    HttpRequest request = new HttpRequest();
    request.onLoadEnd.listen((_) {
      Map<String, dynamic> response = json.decode(request.responseText);
      if(response['success'] != null && response['success']){
        print('login successful');
        window.localStorage['Token'] = response['authToken'];
        window.location.href = routes.values.toUrl();
        // Log in valid, proceed
      } else {
        print('login failed');
      }
    }); 
    
    request.open('POST', _apiEndpoint + 'APILogin');
    request.setRequestHeader('Content-Type', 'application/json');
    request.send(json.encode(body));
  }  

  Future<Null> Unseal() async{
    // Try to unseal with the key
    HttpRequest request = new HttpRequest();
    request.onLoadEnd.listen((_) {
      Map<String, dynamic> response = json.decode(request.response);
      if(response['sealed'] != null && response['sealed']) {
        Keys.add(UnsealKey);
      } else {
        IsSealed = false;
      }
    });

    request.open('POST', _apiEndpoint + 'Unseal');
    request.setRequestHeader('Content-Type', 'application/json');
    request.send(json.encode({"unsealKey" : UnsealKey}));
  }

}