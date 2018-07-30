import 'dart:html';
import 'dart:async';
import 'dart:convert';
import 'package:http/browser_client.dart';
import 'package:angular/angular.dart';
import 'package:angular_router/angular_router.dart';

import 'src/routes.dart';

// AngularDart info: https://webdev.dartlang.org/angular
// Components info: https://webdev.dartlang.org/components

@Component(
  selector: 'my-app',
  templateUrl: 'app_component.html',
  directives: [coreDirectives, routerDirectives],
  providers: [ClassProvider(Routes)]
)

class AppComponent implements OnInit{
  final Routes routes;
  final Router _router;
  AppComponent(this.routes, this._router);

  final  String _logInEndpoint = window.location.origin + '/twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/GetStatus'; 
  checkConn() async {
    // Construct request to twirp server
    var completer = new Completer<bool>();
    HttpRequest connRequest = new HttpRequest();
     connRequest.onLoadEnd.listen((_) {
      Map<String, dynamic> connResponse = json.decode(connRequest.responseText);
      bool isConnected = connResponse['connected'] == null ? false : connResponse['connected'] as bool;
      //print(connResponse);
      print(isConnected);
      if(!isConnected) {
        _router.navigate(routes.reset.toUrl(), NavigationParams(reload: true));
      }
      // Convert null values to false; Extract vault status
      //completer.complete(isConnected);
      //isConnected = connResponse['connected'] == null ? false : connResponse['connected'] as bool;
     });
    connRequest.open('POST', window.location.origin + '/twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/CheckConnection');
    connRequest.setRequestHeader('Content-Type', 'application/json');
    await connRequest.send('{}');
    //return completer.future;
  }
  Future<Null> ngOnInit() async {
    bool isSealed, isInitialized, isConnected;
    //bool isConnected = false;
    bool isAuthorized = false;
    if (window.localStorage["Token"] != null) {
      HttpRequest authRequest = new HttpRequest();
      authRequest.onLoadEnd.listen((_) {
        isAuthorized = authRequest.status != 401;
      });
      authRequest.open('GET', window.location.origin + '/auth');
      authRequest.setRequestHeader("Authorization", "Bearer " + window.localStorage["Token"]);
      await authRequest.send();
    }

     HttpRequest connRequest = new HttpRequest();
     connRequest.onLoadEnd.listen((_) {
      Map<String, dynamic> connResponse = json.decode(connRequest.responseText);
      bool isConnected = connResponse['connected'] == null ? false : connResponse['connected'] as bool;
      print(connResponse);
      print(isConnected);
      if(!isConnected) {
        _router.navigate(routes.reset.toUrl(), NavigationParams(reload: true));
      }
     });
    connRequest.open('POST', window.location.origin + '/twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/CheckConnection');
    connRequest.setRequestHeader('Content-Type', 'application/json');
    await connRequest.send('{}');
    print("isconnected");
    print(isConnected);
    print("isauth");
    print(isAuthorized);

    // Check status of vault and route to proper view
    HttpRequest request = new HttpRequest();
    request.onLoadEnd.listen((_) {
      Map<String, dynamic> response = json.decode(request.responseText);
      print(response);
      // Convert null values to false; Extract vault status
      isSealed = response['sealed'] == null ? false : response['sealed'] as bool;
      isInitialized = response['initialized']==null ? false : response['initialized'] as bool;
      checkConn().then(
        (e) {
        // result = e;
          isConnected = e;
      });
      print("isConnected2");
      print(isConnected);
      //print(isAuthorized);
      if (!isInitialized) { // Vault needs to be seeded
        _router.navigate(routes.sealed.toUrl(), NavigationParams(reload: true));
      } else if (!isAuthorized || isSealed) {  // Vault seeded, user needs to login and recieve token. Vault possibly needs to be unsealed
        print("login");
        _router.navigate(routes.login.toUrl(), NavigationParams(queryParameters: {'sealed': isSealed.toString()}, reload: true));
      } else { // User has auth token and vault is unsealed. Forward to values. May be redirected back to login if token is rejected
        print("values");
        _router.navigate(routes.values.toUrl(), NavigationParams(reload: true));
      }
    });

    // Send to the GetStatus twirp endpoint and wait for response
    request.open('POST', _logInEndpoint);
    request.setRequestHeader('Content-Type', 'application/json');
    await request.send('{}');
  }
  
}
