import 'package:angular/angular.dart';
import 'package:angular_forms/angular_forms.dart';

@Component(
  selector: 'login-box',
  styleUrls: ['login_box_component.css'],
  templateUrl: 'login_box_component.html',
  directives: [CORE_DIRECTIVES, formDirectives]
)

class LoginBoxComponent{
  @Input()
  String Username;
  @Input()
  String Password;
}