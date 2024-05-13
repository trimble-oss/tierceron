
## Contributing to the service

Please take a moment to review this in order to make the contribution process easy and effective.

Following these guidelines helps to communicate that you respect the time of the developers managing and developing this project. 

Contact the TC (trusted commiters) to join the [```Google Chat```](https://chat.google.com/u/1/room/AAAA5cd_TB0) group to find out more information.

### Issue tracker
We use [JIRA issue tracker](
https://jira.trimble.tools/secure/RapidBoard.jspa?projectKey=HON&rapidView=3367&view=planning), being the preferred channel for bug reports, features requests and submitting pull requests.

Every feature/bug created should contain the MANTA Epic Link [MANTA-2194](https://jira.trimble.tools/browse/MANTA-2194)

<br>

### Bugs reporting

A bug is a problem that is caused by the code in the repository. Good bug reports are extremely helpful - but this should be demonstrated.

**Guidelines for bug reports**:
1. Check if the issue has already been reported (check the latest commits and the JIRA board)
2. Check if the issue has been fixed — try to reproduce it using the latest master or develop branch in the repository.
3. If the bug was not already reported, you should create a JIRA bug and send a message on the chat group to inform TC about it.

<br>


Please try to be as detailed as possible in your report:
- Specify the environment
- Steps to reproduce the issue 
- What is the actual result?
- What would you expect to be the outcome? 

All these details will help people to fix any potential bugs.

Example:

> Short example for a bug report
>
> A summary of the issue and the environment in which it occurs. Include the steps required to reproduce the bug.
>
> 1. This is the 1st step
> 2. This is the 2nd step
>
> `<url>` - an optional link with a demo video
>
> You can  include the lines of code that you have identified as causing the bug and potential solutions.

<br>

### Feature requests
Feature requests are always welcome, but before doing thing take a moment to find if your idea fits with the scope of the project. 
It's up to you to make a strong case to convince the project's developers of the merits of this feature. 
Please provide as much detail and context as possible.

<br>

### Pull requests

Pull requests should be focused in scope and to avoid containing unrelated ```commits```.

**Before** creating a PR for a feature/bugfix you should acknowledge and agree the work that will be done it is accepted by the TC (trusted commiters), otherwise you risk spending time working on something that the project's TC  might consider that is not needed for the project.

You also need to know: 
- before creating a pull request, the ```feature or the bugfix``` should be tested by QA on the branch itsself.
- every contributor should have a JIRA item to track the progress.
- the **pull request** should be created after the entire work has been done for your feature/bugfix. In this way, the PR will contain all the commits messages and the Description field will be automatically filled in.
- once you've created a pull request, add the TC (mentioned in [README](README.MD)) as reviewers.
- the pull request will not be merged until is verified by QA.
<br>

### Definition of done
Once a PR is merged to ```develop``` branch, the contributor has to ensure that the ```feature or the bugfix``` is tested by QA on the  ```develop``` branch also to add any comments/progress on the JIRA ticket.
<br>

The item will be really done, once it is merged to develop, contains automated tests, successfully builds and there are no issues on the ```dev environment```.




