import 'package:angular/angular.dart';

// import 'src/login_box/login_box_component.dart';
import 'src/vault_start/vault_start_component.dart';

// AngularDart info: https://webdev.dartlang.org/angular
// Components info: https://webdev.dartlang.org/components

@Component(
  selector: 'my-app',
  styleUrls: ['app_component.css'],
  templateUrl: 'app_component.html',
  directives: [VaultStartComponent],
)
class AppComponent {
  // Nothing here yet. All logic is in TodoListComponent.
}
