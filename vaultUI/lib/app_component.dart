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
  
  Future<Null> ngOnInit() async {
    bool isSealed, isInitialized, isConnected;
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

    print("checking vault");
    var token = "Bearer " + window.localStorage['Token'];
    var client = new BrowserClient();
    var response =
        await client.get(window.location.origin + '/graphql?query={envs{name}}');//, headers: {'Authorization': token});
      if (response.statusCode == 401) { // Unauthorized, redirect to login page
        print("unauthorized");
        window.localStorage.remove("Token");
        window.location.href = routes.login.toUrl();
      }
      Map vaultVals = json.decode(response.body);
      Map data = vaultVals['data'];
      List envs = data['envs'];
      if (envs == null){
        print("envs is null");
        isConnected = false;
        //print(isConnected);
      } else {
        print("envs isn't null");
        isConnected = true;
        //print(isConnected);
      }

    // Check status of vault and route to proper view
    HttpRequest request = new HttpRequest();
    request.onLoadEnd.listen((_) {
      Map<String, dynamic> response = json.decode(request.responseText);
      //print(response);
      // Convert null values to false; Extract vault status
      isSealed = response['sealed'] == null ? false : response['sealed'] as bool;
      isInitialized = response['initialized']==null ? false : response['initialized'] as bool;
      //print(isConnected);
      
      //print(isAuthorized);
      if(!isConnected) {
        _router.navigate(routes.reset.toUrl(), NavigationParams(reload: true));
      } else if (!isInitialized) { // Vault needs to be seeded
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
