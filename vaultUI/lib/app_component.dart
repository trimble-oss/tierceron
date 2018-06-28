import 'package:angular/angular.dart';
import 'dart:html';
import 'dart:async';
import 'dart:convert';

import 'src/login_box/login_box_component.dart';
import 'src/vault_start/vault_start_component.dart';

// AngularDart info: https://webdev.dartlang.org/angular
// Components info: https://webdev.dartlang.org/components

@Component(
  selector: 'my-app',
  styleUrls: ['app_component.css'],
  templateUrl: 'app_component.html',
  directives: [CORE_DIRECTIVES, VaultStartComponent, LoginBoxComponent],
)
class AppComponent implements OnInit{
  // Nothing here yet. All logic is in TodoListComponent.
  bool isSealed;
  bool isInitialized;

  final  String _logInEndpoint = 'http://localhost:8008/twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/GetStatus'; 

  Future<Null> ngOnInit() {
    isInitialized = true;
    isSealed = false;
    checkSeal();
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
      print("Sealed: " + isInitialized.toString());
    });

    request.open('POST', _logInEndpoint);
    request.setRequestHeader('Content-Type', 'application/json');
    request.send('{}');
  }
}
