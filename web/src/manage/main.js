import { mount } from "svelte";
import UserSection from "./UserSection.svelte";
import DangerSection from "./DangerSection.svelte";

var userTarget = document.getElementById("manage-user");
var dangerTarget = document.getElementById("manage-danger");

if (userTarget) mount(UserSection, { target: userTarget });
if (dangerTarget) mount(DangerSection, { target: dangerTarget });
