document.addEventListener("DOMContentLoaded", () => {
  const dropZone = document.getElementById("dropZone");
  const fileInput = document.getElementById("fileInput");
  const loading = document.getElementById("loading");
  const optionsDiv = document.getElementById("options");
  const tableSelect = document.getElementById("tableSelect");
  const newTableName = document.getElementById("newTableName");
  const previewBtn = document.getElementById("previewBtn");
  const previewDiv = document.getElementById("preview");
  const previewTable = document.getElementById("previewTable");
  const commitBtn = document.getElementById("commitBtn");
  let csvFile = null; // Store the uploaded file

  // Event listeners for drag and drop functionality
  dropZone.addEventListener("dragover", (e) => {
    e.preventDefault(); // Prevent default to allow drop
    dropZone.classList.add("dragover"); // Add visual feedback
  });

  dropZone.addEventListener("dragleave", () => {
    dropZone.classList.remove("dragover"); // Remove visual feedback
  });

  dropZone.addEventListener("drop", (e) => {
    e.preventDefault(); // Prevent default behavior
    dropZone.classList.remove("dragover"); // Remove visual feedback
    const files = e.dataTransfer.files;
    if (files.length > 0) {
      handleFile(files[0]);
    }
  });

  // Event listener for manual file input click
  dropZone.addEventListener("click", () => fileInput.click());

  // Event listener for when a file is selected via input
  fileInput.addEventListener("change", (e) => {
    const files = e.target.files;
    if (files.length > 0) {
      handleFile(files[0]);
    }
  });

  // Function to handle the selected file
  function handleFile(file) {
    if (file.type === "text/csv") {
      csvFile = file; // Store the file globally
      uploadCSV(csvFile); // Proceed with upload
    } else {
      alert("Please upload a valid CSV file."); // Inform user
      fileInput.value = ""; // Reset file input
      csvFile = null; // Clear stored file
    }
  }

  // Function to show/hide loading indicator
  function showLoading(show) {
    loading.style.display = show ? "block" : "none";
  }

  // Function to upload the CSV file to the backend
  function uploadCSV(file) {
    showLoading(true); // Show loading indicator
    // Reset UI state
    optionsDiv.style.display = "none";
    previewDiv.style.display = "none";
    tableSelect.innerHTML = '<option value="">-- Select Table --</option>';
    newTableName.value = "";

    const formData = new FormData();
    formData.append("csvfile", file);

    // Fetch request to the upload endpoint
    fetch("/upload", {
      method: "POST",
      body: formData,
    })
      .then(async (response) => {
        if (!response.ok) {
          // Attempt to read error message from response body
          const errorText = await response.text();
          throw new Error(`Failed to upload CSV: ${errorText}`);
        }
        return response.json(); // Parse JSON response
      })
      .then((data) => {
        // Populate table selection dropdown
        data.tableNames.forEach((name) => {
          const option = document.createElement("option");
          option.value = name;
          option.textContent = name;
          tableSelect.appendChild(option);
        });

        // Store parsed CSV data (headers and preview rows) for later use
        // Note: This stores the first 50 rows for preview, not the whole file
        // The whole file is read again on commit.
        window.parsedCSVData = data.CSVData;

        // Show table options section
        optionsDiv.style.display = "block";
        showLoading(false); // Hide loading indicator
      })
      .catch((error) => {
        showLoading(false); // Hide loading indicator
        console.error("Error:", error);
        alert(`Error processing CSV: ${error.message}`); // Display error message
        fileInput.value = ""; // Reset file input on error
        csvFile = null; // Clear stored file on error
      });
  }

  // Event listener for the Preview button
  previewBtn.addEventListener("click", () => {
    if (!csvFile) {
      alert("Please upload a CSV file first.");
      return;
    }

    // Check if a table is selected or a new table name is entered
    const selectedTable = tableSelect.value;
    const newTable = newTableName.value.trim();

    if (!selectedTable && !newTable) {
      alert("Please select an existing table or enter a new table name.");
      return;
    }

    if (selectedTable && newTable) {
      alert(
        "Please either select an existing table OR enter a new table name, not both.",
      );
      return;
    }

    // The parsed CSV data (including preview rows) is already stored in window.parsedCSVData
    const data = window.parsedCSVData;

    if (!data || !data.Headers || !data.Rows) {
      alert("CSV data not available for preview. Please re-upload the file.");
      return;
    }

    const thead = previewTable.querySelector("thead");
    const tbody = previewTable.querySelector("tbody");
    thead.innerHTML = ""; // Clear previous headers
    tbody.innerHTML = ""; // Clear previous rows

    // Populate table header with column names and inferred types
    const headerRow = document.createElement("tr");
    data.Headers.forEach((col) => {
      const th = document.createElement("th");
      th.textContent = `${col.name} (${col.dataType})`;
      headerRow.appendChild(th);
    });
    thead.appendChild(headerRow);

    // Populate table body with preview rows (first 50)
    data.Rows.forEach((row) => {
      const tr = document.createElement("tr");
      row.forEach((cell) => {
        const td = document.createElement("td");
        td.textContent = cell; // Display cell value
        tr.appendChild(td);
      });
      tbody.appendChild(tr);
    });

    // Show the preview section
    previewDiv.style.display = "block";
  });

  // Event listener for the Commit button
  commitBtn.addEventListener("click", () => {
    if (!csvFile) {
      alert("No CSV file uploaded.");
      return;
    }

    const tableName = tableSelect.value || newTableName.value.trim();
    const action = tableSelect.value
      ? document.querySelector('input[name="action"]:checked').value // 'append' or 'overwrite' for existing
      : "create"; // 'create' for a new table

    if (!tableName) {
      alert("Please select a table or enter a new table name.");
      return;
    }

    if (tableSelect.value && newTableName.value.trim()) {
      alert(
        "Please either select an existing table OR enter a new table name, not both.",
      );
      return;
    }

    showLoading(true); // Show loading indicator

    const formData = new FormData();
    formData.append("csvfile", csvFile); // Append the whole CSV file
    formData.append(
      "commitData",
      JSON.stringify({ tableName: tableName, action: action }), // Append commit options as JSON string
    );

    // Fetch request to the commit endpoint
    fetch("/commit", {
      method: "POST",
      body: formData,
    })
      .then(async (response) => {
        if (!response.ok) {
          // Attempt to read error message from response body
          const errorText = await response.text();
          throw new Error(`Failed to commit data: ${errorText}`);
        }
        return response.text(); // Expect text response
      })
      .then((message) => {
        showLoading(false); // Hide loading indicator
        alert(message); // Show success message from backend
        // Reset UI after successful commit
        optionsDiv.style.display = "none";
        previewDiv.style.display = "none";
        fileInput.value = ""; // Clear file input
        csvFile = null; // Clear stored file
        window.parsedCSVData = null; // Clear stored data
      })
      .catch((error) => {
        showLoading(false); // Hide loading indicator
        console.error("Error:", error);
        alert(`Error committing data: ${error.message}`); // Display error message
      });
  });

  // Event listeners to handle exclusivity between tableSelect and newTableName input
  tableSelect.addEventListener("change", () => {
    if (tableSelect.value) {
      newTableName.disabled = true; // Disable new table input if existing is selected
      newTableName.value = ""; // Clear new table name
    } else {
      newTableName.disabled = false; // Enable new table input
    }
  });

  newTableName.addEventListener("input", () => {
    if (newTableName.value.trim()) {
      tableSelect.disabled = true; // Disable table select if new table name is entered
      tableSelect.value = ""; // Reset selected table
      // Also disable append/overwrite radio buttons as action will be 'create'
      document.querySelectorAll('input[name="action"]').forEach((radio) => {
        radio.disabled = true;
        if (radio.value === "create") {
          radio.checked = true;
        } else {
          radio.checked = false; // Uncheck append/overwrite
        }
      });
    } else {
      tableSelect.disabled = false; // Enable table select
      // Enable append/overwrite radio buttons
      document.querySelectorAll('input[name="action"]').forEach((radio) => {
        radio.disabled = false;
        if (radio.value === "append") {
          radio.checked = true; // Default to append
        } else {
          radio.checked = false; // Uncheck create/overwrite initially
        }
      });
    }
  });

  // Initial state check in case of browser autofill
  if (newTableName.value.trim()) {
    tableSelect.disabled = true;
    document.querySelectorAll('input[name="action"]').forEach((radio) => {
      radio.disabled = true;
      if (radio.value === "create") {
        radio.checked = true;
      } else {
        radio.checked = false;
      }
    });
  }
});
