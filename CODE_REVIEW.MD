
## Code Review

### Guidelines ✍ 
Code review is required to get a second opinion on the chosen solution and
implementation, having an extra pair of eyes looking for bugs, logic problems, or
uncovered edge cases and also to ensure that the code is properly constructed.

So, our first step toward better code is to review it.

To make it easier to review code, always work in a separate branch. The branch reduces the temptation to push unreviewed code or to wait too long to push code.

You should be the first person that review every line of your code. Before committing new code, read each changed line. Use ```git's diff ``` feature to examine code before you commit. 

This is a list of requirements that you should expect for **code reviews**:

- commits messages should describe the actual changes
- all commits should be prefixed with the relevant JIRA issue number (i.e commit message: *HON-1515: added new UTs for login method*)
- follow the coding standards rules mentioned below
- submitted code should be covered by Unit Tests and Integration Tests

**Other expectations**:
- contact the TC for the code review process
- expect on turnaround time for code review
- on the PR page in bitbucket, you will receive feedback (using comments option) and tasks (that should be made so the PR can be merged)


### Coding Standards ⌨
- keep the document formatted (in VSCode, this should happen automatically.)
- you can add comments in the code if there is something that is not obvious and is worth mentioning
- every ```if statements``` should contain parentheses (even if the block instruction has only one line of code)
- try to use ```var``` keyword just for unavoidable cases. Instead, specify the type of the object used.


You can also read [Effective Go](https://golang.org/doc/effective_go)


