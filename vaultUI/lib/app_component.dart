import 'dart:html';
import 'dart:async';
import 'dart:convert';

import 'package:angular/angular.dart';
import 'package:angular_router/angular_router.dart';

import 'src/routes.dart';
import 'src/login_box/login_box_component.dart';
import 'src/vault_start/vault_start_component.dart';
import 'src/vault_vals/vault_vals_component.dart';

// AngularDart info: https://webdev.dartlang.org/angular
// Components info: https://webdev.dartlang.org/components

@Component(
  selector: 'my-app',
  templateUrl: 'app_component.html',
  directives: [coreDirectives, 
               routerDirectives, 
               LoginBoxComponent, 
               VaultStartComponent,
               VaultValsComponent],
  providers: [ClassProvider(Routes)]
)
class AppComponent implements OnInit{
  // Nothing here yet. All logic is in TodoListComponent.
  bool isSealed;
  bool isInitialized;

  final Routes routes;
  AppComponent(this.routes);

  final  String _logInEndpoint = 'http://localhost:8008/twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/GetStatus'; 

  Future<Null> ngOnInit() {
    isInitialized = true;
    isSealed = false;
    checkSeal();
    return null;
  }

  Future<Null> checkSeal() async {
    HttpRequest request = new HttpRequest();
    request.onLoadEnd.listen((_) {
      Map<String, dynamic> response = json.decode(request.responseText);
      if(response['sealed'] == null) {
        isSealed = false;
      } else {
        isSealed = response['sealed'];
      }
      
      if(response['initialized']==null){
        isInitialized = false;
      } else {
        isInitialized = response['initialized'];
      }
      print("Initialized: " + isInitialized.toString());
      print("Sealed: " + isSealed.toString());
    });

    request.open('POST', _logInEndpoint);
    request.setRequestHeader('Content-Type', 'application/json');
    request.send('{}');
  }
}
