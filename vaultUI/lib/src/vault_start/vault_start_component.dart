import 'package:angular/angular.dart';
import 'package:angular_forms/angular_forms.dart';

import 'dart:html';
import 'dart:core';
import 'dart:async';
// Displayed when vault has not yet been created,w

@Component(
  selector: 'vault-start',
  styleUrls: ['vault_start_component.css'],
  templateUrl: 'vault_start_component.html',
  directives: [CORE_DIRECTIVES, formDirectives]
)

class VaultStartComponent{
  List<String> Envs = ['local', 'dev', 'QA'];
  String Address = 'https://127.0.0.1:8200';
  Set<File> Seeds = Set.identity();
  @Input()
  FileList SeedBuffer;
  @Input()
  String Username;
  @Input()
  String Password;

  GetFiles(event) {
    querySelector('#no_seed_warn').hidden = true;
    RegExp ext = new RegExp(r'(.+\.yml|.+\.yaml)');
    this.SeedBuffer = event.target.files;
    SeedBuffer.forEach((file) => print(file));
    print('===Current File Set===');
    SeedBuffer.forEach((bufferfile){
      if (!ext.hasMatch(bufferfile.name)) return; // Make sure file is yaml
      bool fileExists = false;
      Seeds.forEach((seedfile) =>
        fileExists = fileExists || seedfile.name == bufferfile.name
      );
      if (!fileExists) Seeds.add(bufferfile);
    });
    Seeds.forEach((seedfile) => (print(seedfile.name)));
  }

  RemoveFile(File seed) {
    Seeds.remove(seed);
  }

  StartVault() {
    print('Starting vault');
    bool valid = true;
    if(Username == null || Username.length == 0){ // Signify user/pass needed
      valid = false;
      querySelector('#username').classes.addAll(['error', 'error_text']);
    } 
    if(Password == null || Password.length == 0) { 
      valid = false;
      querySelector('#password').classes.addAll(['error', 'error_text']);
    }
    if(Seeds == null || Seeds.length == 0) {
      valid = false;
      print('No seeds');
      querySelector('#no_seed_warn').hidden = false;
    }
  }

  Future<Null> UnRedify(event) async{
    List<String> removals  = ['error', 'error_text'];
    (event.target as Element).classes.removeAll(removals);
  }
}