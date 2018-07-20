import 'dart:html';
import 'dart:async';
import 'dart:convert';

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
    bool isSealed, isInitialized;

    // Check status of vault and route to proper view
    HttpRequest request = new HttpRequest();
    request.onLoadEnd.listen((_) {
      Map<String, dynamic> response = json.decode(request.responseText);
      
      // Convert null values to false; Extract vault status
      isSealed = response['sealed'] == null ? false : response['sealed'] as bool;
      isInitialized = response['initialized']==null ? false : response['initialized'] as bool;

      if (!isInitialized) { // Vault needs to be seeded
        _router.navigate(routes.sealed.toUrl(), NavigationParams(reload: true));
      } else if (!window.localStorage.containsKey("Token") || isSealed) {  // Vault seeded, user needs to login and recieve token. Vault possibly needs to be unsealed
        _router.navigate(routes.login.toUrl(), NavigationParams(queryParameters: {'sealed': isSealed.toString()}, reload: true));
      } else { // User has auth token and vault is unsealed. Forward to values. May be redirected back to login if token is rejected
        _router.navigate(routes.values.toUrl(), NavigationParams(reload: true));
      }
    });

    // Send to the GetStatus twirp endpoint and wait for response
    request.open('POST', _logInEndpoint);
    request.setRequestHeader('Content-Type', 'application/json');
    await request.send('{}');
  }
}
