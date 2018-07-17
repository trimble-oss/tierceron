import 'dart:html';
import 'dart:async';
import 'dart:convert';

import 'package:angular/angular.dart';
import 'package:angular_router/angular_router.dart';

import 'src/routes.dart';
//import 'src/login_box/login_box_component.dart';
//import 'src/vault_start/vault_start_component.dart';
import 'src/vault_vals/vault_vals_component.dart';

// AngularDart info: https://webdev.dartlang.org/angular
// Components info: https://webdev.dartlang.org/components

@Component(
  selector: 'my-app',
  templateUrl: 'app_component.html',
  directives: [coreDirectives, routerDirectives, VaultValsComponent], //LoginBoxComponent, VaultStartComponent],
  providers: [ClassProvider(Routes)]
)
class AppComponent implements OnInit{
  // Nothing here yet. All logic is in TodoListComponent.
  final Routes routes;
  final Router _router;
  AppComponent(this.routes, this._router);

  final  String _logInEndpoint = window.location.origin + '/twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/GetStatus'; 

  Future<Null> ngOnInit() async {
    bool isSealed, isInitialized;

    HttpRequest request = new HttpRequest();
    request.onLoadEnd.listen((_) {
      Map<String, dynamic> response = json.decode(request.responseText);
      
      if(response['sealed'] == null) {
        isSealed = false;
      } else {
        isSealed = response['sealed'] as bool;
      }
      
      if(response['initialized']==null){
        isInitialized = false;
      } else {
        isInitialized = response['initialized'] as bool;
      }

      if (!isInitialized) {
        _router.navigate(routes.sealed.toUrl(), NavigationParams(reload: true));
      } else if (!window.localStorage.containsKey('Token') || isSealed) {
        _router.navigate(routes.login.toUrl(), NavigationParams(queryParameters: {'sealed': isSealed.toString()}, reload: true));
      } else {
        _router.navigate(routes.values.toUrl(), NavigationParams(reload: true));
      }
    });

    request.open('POST', _logInEndpoint);
    request.setRequestHeader('Content-Type', 'application/json');
    await request.send('{}');
  }

  Future<Null> checkSeal() async {
    
  }
}
