import 'package:angular/angular.dart';
import 'package:angular_forms/angular_forms.dart';
import 'package:angular_components/angular_components.dart';

import 'dart:async';
import 'dart:html';
import 'dart:convert';

@Component(
  selector: 'login-box',
  styleUrls: ['login_box_component.css'],
  templateUrl: 'login_box_component.html',
  directives: const [CORE_DIRECTIVES, 
                     formDirectives,
                     ModalComponent],
  providers: const [materialProviders]

)

class LoginBoxComponent{
  @Input()
  String Username;
  @Input()
  String Password;

  @Input()
  bool IsSealed;
  @Input()
  String UnsealKey;

  final String _logInEndpoint = 'http://localhost:8008/twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/APILogin';   // Vault addreess

  SignIn() {
    Map<String, dynamic> body = new Map();
    body['username'] = Username;
    body['password'] = Password;
    
    HttpRequest request = new HttpRequest();
    request.onLoadEnd.listen((_) {
      Map<String, dynamic> response = json.decode(request.responseText);
      if(response['success'] != null && response['success']){
        print('login successful');
        // Log in valid, proceed
      } else {
        print('login failed');
      }
    }); 
    
    request.open('POST', _logInEndpoint);
    request.setRequestHeader('Content-Type', 'application/json');
    request.setRequestHeader('Authorization', 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIzMjE2NTQ5ODcwIiwibmFtZSI6IldlYiBBcHAiLCJpYXQiOjE1MTYyMzkwMjIsImlzcyI6IlZpZXdwb2ludCwgSW5jLiIsImF1ZCI6IlZpZXdwb2ludCBWYXVsdCBXZWJBUEkifQ.ls2cxzqIMv3_72BKH9K34uR-gWgeFTGqu-tXGh503Jg');
    request.send(json.encode(body));
  }  

}