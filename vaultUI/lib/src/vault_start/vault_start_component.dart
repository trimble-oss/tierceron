import 'dart:html';
import 'dart:core';
import 'dart:async';
import 'dart:convert';

import 'package:angular/angular.dart';
import 'package:angular_forms/angular_forms.dart';
import 'package:cryptoutils/cryptoutils.dart';

import '../twirp_requests.dart';
// Displayed when vault has not yet been created,w

@Component(
  selector: 'vault-start',
  styleUrls: ['vault_start_component.css'],
  templateUrl: 'vault_start_component.html',
  directives: [CORE_DIRECTIVES, formDirectives],
  pipes: const [SlicePipe]
)

class VaultStartComponent{
  final List<String> Envs = ['local', 'dev', 'QA'];  // Valid environment options
  final String Address = 'https://127.0.0.1:8200';   // Vault addrees 
  Set<File> Seeds = Set.identity();                  // Seed files passed to vault
  Set<SeedFile> base64files = Set.identity();        // List of files in twirp reqeust format to be sent
  @Input()
  FileList SeedBuffer;  // MODEL: <input> for file upload under #file_list
  @Input()
  String Username;      // MODEL: <input> for new username under login_creation
  @Input()
  String Password;      // MODEL: <input> for new password under login_creation

  // Callback for file input element
  GetFiles(event) {
    // Ensure warning is hidden if new files have been chosen
    querySelector('#no_seed_warn').hidden = true;
    RegExp ext = new RegExp(r'(.+\.yml|.+\.yaml)'); 
    this.SeedBuffer = event.target.files; // Get files from html element
    SeedBuffer.forEach((file) => print(file));
    print('===Current File Set==='); // Log files
    SeedBuffer.forEach((bufferfile){
      if (!ext.hasMatch(bufferfile.name)) return; // Make sure file is yaml
      bool fileExists = false;
      Seeds.forEach((seedfile) => // Skip duplicates
        fileExists = fileExists || seedfile.name == bufferfile.name
      );
      if (!fileExists) Seeds.add(bufferfile); // Display newly added file
    });
    Seeds.forEach((seedfile) => (print(seedfile.name)));
  }

  // Used for remove file button
  RemoveFile(File seed) {
    Seeds.remove(seed);
  }

  // Used to send seed files and start vault
  StartVault() {
    print('Starting vault');
    bool valid = true;
    if(Username == null || Username.length == 0){ // Check username exists
      valid = false;
      querySelector('#username').classes.addAll(['error', 'error_text']);
    } 
    if(Password == null || Password.length == 0) { // Check password exists
      valid = false;
      querySelector('#password').classes.addAll(['error', 'error_text']);
    }
    if(Seeds == null || Seeds.length == 0) { // Check at least one seed file given
      valid = false;
      print('No seeds');
      querySelector('#no_seed_warn').hidden = false;
    }
    if(valid){ // Username, Password, files given; Begin init
      // Proceed to seed vault and change layout

      // Iterate through files, converting to base 64
      Seeds.forEach((file) {
        FileReader f = new FileReader(); // Parse file data
        String env = (querySelector('#' + file.name.substring(0, file.name.length-4)) 
                      as SelectElement).value;
        // Callback for successful read
        f.onLoad.listen((fileEvent) {
          // Convert to base64, fetch file name and environment
          print('Reading ' + file.name);
          List<int> fileBytes = utf8.encode(f.result);
          base64files.add(new SeedFile(file.name, CryptoUtils.bytesToBase64(fileBytes), env));
          print(base64files.last.name + ' ' + base64files.last.env);
          print(base64files.last.data);
        });
        f.onError.listen((errorEvent) => print(errorEvent));

        f.readAsText(file);
      });

      // POST: https://<apirouterhost>:<port>/twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/InitVault
      // new=true, seeds=seeds, 
    }
  }


  // Remove error formatting from username/password box
  Future<Null> UnRedify(event) async{
    List<String> removals  = ['error', 'error_text'];
    (event.target as Element).classes.removeAll(removals);
  }
}