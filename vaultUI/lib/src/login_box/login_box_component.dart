import 'package:angular/angular.dart';
import 'package:angular_forms/angular_forms.dart';
import 'package:angular_components/angular_components.dart';

import 'dart:async';

@Component(
  selector: 'login-box',
  styleUrls: ['login_box_component.css'],
  templateUrl: 'login_box_component.html',
  directives: const [CORE_DIRECTIVES, 
                     formDirectives,
                     ModalComponent],
  providers: const [materialProviders]

)

class LoginBoxComponent implements OnInit{
  @Input()
  String Username;
  @Input()
  String Password;

  bool isSealed;
  @Input()
  String UnsealKey;

  Future<Null> ngOnInit() async{
    // Check vault health to see if sealed 
    isSealed=false;
  }

  SignIn() {

  }

}