import 'package:angular/angular.dart';
import 'package:angular_forms/angular_forms.dart';
import 'package:angular_components/angular_components.dart';
import 'package:angular_router/angular_router.dart';

import '../routes.dart';
import 'dart:async';
import 'dart:html';
import 'dart:convert';

@Component(
  selector: 'server-reset',
  styleUrls: ['server_reset_component.css'],
  templateUrl: 'server_reset_component.html',
  directives: const [coreDirectives,
                     formDirectives,
                     routerDirectives,
                     ModalComponent],
  providers: const [materialProviders, ClassProvider(Routes)]

)

class ServerResetComponent {

  @Input()
  String Token;

  Set<String> Keys = new Set();

  final Routes routes;
  final Router _router;
  ServerResetComponent(this.routes, this._router);

  // Future<Null> onActivate(_, RouterState current) async {
  //   IsSealed = current.queryParameters['sealed'].toLowerCase() == 'true';
  // }

  final String _apiEndpoint = 'http://127.0.0.1:8008/twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/';   // Vault addreess

  Future<Null> RestartServer() async{
    // Fetch input token for making the request.
    Map<String, dynamic> body = new Map();
    body['token'] = Token;

    // Construct request to twirp server
    HttpRequest request = new HttpRequest();
    request.onLoadEnd.listen((_) {
      body.remove('token'); // Clear token
      Token = '';
      RouteToLogin();
    }); 
    
    request.open('POST', _apiEndpoint + 'ResetServer');
    request.setRequestHeader('Content-Type', 'application/json');
    request.send(json.encode(body));
  }  
  RouteToLogin()async{
    //sign out and redirect to login page
    bool isSealed;
    final  String _logInEndpoint = 'http://127.0.0.1:8008/twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/GetStatus'; 
    HttpRequest request = new HttpRequest();
    request.onLoadEnd.listen((_) {
      Map<String, dynamic> response = json.decode(request.responseText);
      // Convert null values to false; Extract vault status
      isSealed = response['sealed'] == null ? false : response['sealed'] as bool;
    
      //print("sealed: " + isSealed.toString());
      // Vault seeded, user needs to login and recieve token. Vault possibly needs to be unsealed
      print("logout");
      window.localStorage.clear();
      _router.navigate(routes.login.toUrl(), NavigationParams(queryParameters: {'sealed': isSealed.toString()}, reload: true));
    });
    request.open('POST', _logInEndpoint);
    request.setRequestHeader('Content-Type', 'application/json');
    await request.send('{}');
  }

}